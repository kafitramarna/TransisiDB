package proxy

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/kafitramarna/TransisiDB/internal/config"
	"github.com/kafitramarna/TransisiDB/internal/dualwrite"
	"github.com/kafitramarna/TransisiDB/internal/logger"
	"github.com/kafitramarna/TransisiDB/internal/parser"
	"github.com/kafitramarna/TransisiDB/pkg/protocol"
)

// Session manages a client connection
type Session struct {
	clientConn   net.Conn
	backendConn  *BackendConn
	config       *config.Config
	backendPool  *BackendPool
	orchestrator *dualwrite.Orchestrator
	parser       *parser.Parser
	connID       uint32
	database     string
	inTx         bool
}

// NewSession creates a new session
func NewSession(conn net.Conn, cfg *config.Config, pool *BackendPool) *Session {
	return &Session{
		clientConn:  conn,
		config:      cfg,
		backendPool: pool,
		connID:      1, // TODO: Generate unique ID from server
	}
}

// Handle processes the session
func (s *Session) Handle() error {
	logger.Info("New connection", "remote_addr", s.clientConn.RemoteAddr().String())
	defer s.clientConn.Close()

	var err error

	// 1. Acquire backend connection from pool or create new one
	if s.backendPool != nil {
		s.backendConn, err = s.backendPool.Acquire()
	} else {
		// Fallback: create direct connection if no pool
		s.backendConn, err = s.createDirectBackendConnection()
	}

	if err != nil {
		return fmt.Errorf("failed to acquire backend connection: %w", err)
	}
	defer s.releaseBackendConnection()

	// Initialize parser and orchestrator
	s.parser = parser.NewParser(s.config.Tables)

	// 2. Proxy Handshake (Backend -> Client)
	handshakePkt, err := protocol.ReadPacket(s.backendConn.Conn())
	if err != nil {
		return fmt.Errorf("failed to read backend handshake: %w", err)
	}

	if err := protocol.WritePacket(s.clientConn, handshakePkt.SequenceID, handshakePkt.Payload); err != nil {
		return fmt.Errorf("failed to forward handshake to client: %w", err)
	}

	// 3. Proxy Auth Response (Client -> Backend)
	authPkt, err := protocol.ReadPacket(s.clientConn)
	if err != nil {
		return fmt.Errorf("failed to read client handshake response: %w", err)
	}

	if err := protocol.WritePacket(s.backendConn.Conn(), authPkt.SequenceID, authPkt.Payload); err != nil {
		return fmt.Errorf("failed to forward auth response to backend: %w", err)
	}

	// 4. Auth Loop (Handle Auth Switch / More Data)
	for {
		authResultPkt, err := protocol.ReadPacket(s.backendConn.Conn())
		if err != nil {
			return fmt.Errorf("failed to read backend auth result: %w", err)
		}

		if err := protocol.WritePacket(s.clientConn, authResultPkt.SequenceID, authResultPkt.Payload); err != nil {
			return fmt.Errorf("failed to forward auth result to client: %w", err)
		}

		if len(authResultPkt.Payload) > 0 {
			pktType := authResultPkt.Payload[0]

			// OK Packet -> Auth Success
			if protocol.IsOKPacket(authResultPkt.Payload) {
				logger.Info("Handshake completed successfully", "conn_id", s.connID)
				break
			}

			// ERR Packet -> Auth Failed
			if protocol.IsERRPacket(authResultPkt.Payload) {
				return fmt.Errorf("authentication failed")
			}

			// Auth Switch Request (0xFE) or Auth More Data (0x01)
			if pktType == 0xFE || pktType == 0x01 {
				logger.Debug("Handling Auth Switch/More Data", "type", fmt.Sprintf("0x%X", pktType))

				clientAuthPkt, err := protocol.ReadPacket(s.clientConn)
				if err != nil {
					return fmt.Errorf("failed to read client auth response: %w", err)
				}

				if err := protocol.WritePacket(s.backendConn.Conn(), clientAuthPkt.SequenceID, clientAuthPkt.Payload); err != nil {
					return fmt.Errorf("failed to forward client auth response to backend: %w", err)
				}
				continue
			}
		}
	}

	// 5. Command Loop
	return s.handleCommands()
}

// handleCommands processes client commands
func (s *Session) handleCommands() error {
	for {
		// Note: We don't set aggressive deadlines here because:
		// - Client may be slow between commands (legitimate idle time)
		// - Backend queries can take variable time
		// - TCP keep-alive (set in listener) handles dead connections
		// - Only set deadline if we need to enforce a specific timeout

		// Read Command from Client
		cmdPkt, err := protocol.ReadPacket(s.clientConn)
		if err != nil {
			return fmt.Errorf("read command error: %w", err)
		}

		if len(cmdPkt.Payload) == 0 {
			continue
		}

		cmd := cmdPkt.Payload[0]
		cmdName := protocol.GetCommandName(cmd)
		logger.Debug("Received command", "command", cmdName, "conn_id", s.connID)

		// Handle different command types
		switch cmd {
		case protocol.COM_QUIT:
			logger.Info("Client requested disconnect", "conn_id", s.connID)
			return nil

		case protocol.COM_PING:
			// Forward ping to backend
			if err := s.forwardCommand(cmdPkt); err != nil {
				return err
			}

		case protocol.COM_INIT_DB:
			// Track database change
			if len(cmdPkt.Payload) > 1 {
				s.database = string(cmdPkt.Payload[1:])
				s.backendConn.SetDatabase(s.database)
				logger.Info("Database changed", "database", s.database, "conn_id", s.connID)
			}
			if err := s.forwardCommand(cmdPkt); err != nil {
				return err
			}

		case protocol.COM_QUERY:
			if err := s.handleQuery(cmdPkt); err != nil {
				return err
			}

		default:
			// Forward unknown commands as-is
			if err := s.forwardCommand(cmdPkt); err != nil {
				return err
			}
		}
	}
}

// handleQuery processes a COM_QUERY command
func (s *Session) handleQuery(cmdPkt *protocol.Packet) error {
	query := string(cmdPkt.Payload[1:])
	logger.Info("Received query", "query", query, "conn_id", s.connID)

	// Track transaction state
	upperQuery := strings.ToUpper(strings.TrimSpace(query))
	if upperQuery == "BEGIN" || upperQuery == "START TRANSACTION" {
		s.inTx = true
		s.backendConn.SetInTransaction(true)
		logger.Debug("Transaction started", "conn_id", s.connID)
	} else if upperQuery == "COMMIT" || upperQuery == "ROLLBACK" {
		s.inTx = false
		s.backendConn.SetInTransaction(false)
		logger.Debug("Transaction ended", "conn_id", s.connID, "command", upperQuery)
	}

	// Parse query
	pq, err := s.parser.Parse(query)
	if err != nil {
		logger.Warn("Failed to parse query", "error", err, "query", query)
		// Forward original query if parsing fails
		return s.forwardCommand(cmdPkt)
	}

	// Check if query needs transformation
	if !pq.NeedsTransform {
		logger.Debug("Query does not need transformation", "query_type", pq.Type)
		return s.forwardCommand(cmdPkt)
	}

	logger.Info("Query needs transformation", "table", pq.TableName, "query_type", pq.Type)

	// Convert currency values
	convertedValues := make(map[string]float64)
	for col, val := range pq.Values {
		var floatVal float64
		if strVal, ok := val.(string); ok {
			fmt.Sscanf(strVal, "%f", &floatVal)
			// Apply conversion ratio and rounding
			convertedVal := floatVal / float64(s.config.Conversion.Ratio)
			convertedValues[col] = convertedVal
		}
	}

	// Rewrite query with shadow columns
	newQuery, err := s.parser.RewriteForDualWrite(pq, convertedValues)
	if err != nil {
		logger.Error("Failed to rewrite query", "error", err)
		return s.forwardCommand(cmdPkt)
	}

	logger.Info("Rewrote query", "original", query, "new", newQuery)

	// Create new packet with rewritten query
	newPayload := make([]byte, 1+len(newQuery))
	newPayload[0] = protocol.COM_QUERY
	copy(newPayload[1:], newQuery)

	// Forward rewritten command
	rewrittenPkt := &protocol.Packet{
		SequenceID: cmdPkt.SequenceID,
		Payload:    newPayload,
	}

	return s.forwardCommand(rewrittenPkt)
}

// forwardCommand forwards a command to backend and proxies response
func (s *Session) forwardCommand(cmdPkt *protocol.Packet) error {
	// Set write deadline
	s.backendConn.Conn().SetWriteDeadline(time.Now().Add(s.config.Proxy.WriteTimeout))

	// Forward command to backend
	if err := protocol.WritePacket(s.backendConn.Conn(), cmdPkt.SequenceID, cmdPkt.Payload); err != nil {
		return fmt.Errorf("failed to forward command to backend: %w", err)
	}

	// Proxy response back to client
func (s *Session) createDirectBackendConnection() (*BackendConn, error) {
	backendDSN := net.JoinHostPort(s.config.Database.Host, fmt.Sprintf("%d", s.config.Database.Port))
	backendConn, err := net.DialTimeout("tcp", backendDSN, s.config.Database.ConnectionTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to backend %s: %w", backendDSN, err)
	}
	return NewBackendConn(backendConn, s.connID), nil
}

// releaseBackendConnection releases the backend connection back to pool or closes it
func (s *Session) releaseBackendConnection() {
	if s.backendConn == nil {
		return
	}

	// Update last used timestamp
	s.backendConn.UpdateLastUsed()

	if s.backendPool != nil {
		s.backendPool.Release(s.backendConn)
	} else {
		s.backendConn.Close()
	}
}
