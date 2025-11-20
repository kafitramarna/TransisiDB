package protocol

import (
	"crypto/rand"
)

// HandshakeV10 represents the initial handshake packet from server to client
type HandshakeV10 struct {
	ProtocolVersion uint8
	ServerVersion   string
	ConnectionID    uint32
	AuthPluginData  []byte
	CapabilityFlags uint32
	CharacterSet    uint8
	StatusFlags     uint16
	AuthPluginName  string
}

// NewHandshakeV10 creates a new default handshake packet
func NewHandshakeV10(connectionID uint32) *HandshakeV10 {
	// Generate random salt
	salt := make([]byte, 20)
	rand.Read(salt)

	return &HandshakeV10{
		ProtocolVersion: 10,
		ServerVersion:   "8.0.30-TransisiDB",
		ConnectionID:    connectionID,
		AuthPluginData:  salt,
		CapabilityFlags: 65535, // Support everything for now
		CharacterSet:    45,    // utf8mb4_general_ci
		StatusFlags:     2,     // SERVER_AUTOCOMMIT
		AuthPluginName:  "mysql_native_password",
	}
}

// Encode serializes the handshake packet
func (h *HandshakeV10) Encode() []byte {
	var buf []byte

	buf = append(buf, h.ProtocolVersion)
	buf = WriteString(buf, h.ServerVersion)
	buf = WriteUint32(buf, h.ConnectionID)

	// Auth Plugin Data Part 1 (8 bytes)
	buf = append(buf, h.AuthPluginData[:8]...)
	buf = append(buf, 0x00) // Filler

	// Capability Flags Lower 2 bytes
	buf = WriteUint16(buf, uint16(h.CapabilityFlags))

	buf = append(buf, h.CharacterSet)
	buf = WriteUint16(buf, h.StatusFlags)

	// Capability Flags Upper 2 bytes
	buf = WriteUint16(buf, uint16(h.CapabilityFlags>>16))

	// Auth Plugin Data Length
	buf = append(buf, 21) // Length of auth plugin data (8 + 12 + 1)

	// Reserved (10 bytes)
	buf = append(buf, make([]byte, 10)...)

	// Auth Plugin Data Part 2 (12 bytes)
	buf = append(buf, h.AuthPluginData[8:20]...)
	buf = append(buf, 0x00) // Null terminator for auth plugin data

	// Auth Plugin Name
	buf = WriteString(buf, h.AuthPluginName)

	return buf
}

// HandshakeResponse41 represents the client's response to handshake
type HandshakeResponse41 struct {
	CapabilityFlags uint32
	MaxPacketSize   uint32
	CharacterSet    uint8
	Username        string
	AuthResponse    []byte
	Database        string
	AuthPluginName  string
}

// DecodeHandshakeResponse41 parses the client handshake response
func DecodeHandshakeResponse41(payload []byte) (*HandshakeResponse41, error) {
	// TODO: Implement decoding logic
	// For MVP we might just proxy this directly to backend
	return &HandshakeResponse41{}, nil
}
