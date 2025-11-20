package proxy

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/kafitramarna/TransisiDB/internal/config"
	"github.com/kafitramarna/TransisiDB/internal/logger"
)

// Server represents the proxy server
type Server struct {
	config      *config.Config
	listener    net.Listener
	backendPool *BackendPool
	mu          sync.Mutex
	running     bool
	wg          sync.WaitGroup
	connSem     chan struct{} // Semaphore for connection limits
}

// NewServer creates a new proxy server
func NewServer(cfg *config.Config) *Server {
	// Create backend pool
	backendPool, err := NewBackendPool(cfg, cfg.Proxy.PoolSize)
	if err != nil {
		logger.Error("Failed to create backend pool", "error", err)
		// Continue without pool, will create connections on-demand
	}

	// Create connection semaphore for max connections limit
	connSem := make(chan struct{}, cfg.Proxy.MaxConnectionsPerHost)

	return &Server{
		config:      cfg,
		backendPool: backendPool,
		connSem:     connSem,
	}
}

// Start starts the proxy server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Proxy.Host, s.config.Proxy.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.mu.Lock()
	s.listener = ln
	s.running = true
	s.mu.Unlock()

	logger.Info("Proxy server listening", "address", addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			s.mu.Lock()
			running := s.running
			s.mu.Unlock()
			if !running {
				return nil
			}
			logger.Error("Accept error", "error", err)
			continue
		}

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// Stop stops the proxy server
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	if s.listener != nil {
		s.listener.Close()
	}

	// Close backend pool
	if s.backendPool != nil {
		s.backendPool.Close()
	}

	s.wg.Wait()
	logger.Info("Proxy server stopped gracefully")
}

func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	// Acquire connection slot (enforce max connections)
	s.connSem <- struct{}{}
	defer func() { <-s.connSem }()

	// Enable TCP keep-alive
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	// Note: We don't set read/write deadlines here because:
	// 1. Handshake needs variable time depending on auth method
	// 2. Deadlines are refreshed in handleCommands() for each command
	// 3. Setting them too early causes "i/o timeout" during auth

	session := NewSession(conn, s.config, s.backendPool)
	if err := session.Handle(); err != nil {
		logger.Error("Session error", "remote_addr", conn.RemoteAddr().String(), "error", err)
	}
}
