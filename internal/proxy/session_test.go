package proxy

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/kafitramarna/TransisiDB/internal/config"
)

type MockConn struct {
	ReadBuf  *bytes.Buffer
	WriteBuf *bytes.Buffer
}

func NewMockConn() *MockConn {
	return &MockConn{
		ReadBuf:  new(bytes.Buffer),
		WriteBuf: new(bytes.Buffer),
	}
}

func (m *MockConn) Read(b []byte) (n int, err error) {
	return m.ReadBuf.Read(b)
}

func (m *MockConn) Write(b []byte) (n int, err error) {
	return m.WriteBuf.Write(b)
}

func (m *MockConn) Close() error                       { return nil }
func (m *MockConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (m *MockConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (m *MockConn) SetDeadline(t time.Time) error      { return nil }
func (m *MockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *MockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestNewSession(t *testing.T) {
	cfg := &config.Config{}
	conn := NewMockConn()

	session := NewSession(conn, cfg, nil)

	if session == nil {
		t.Error("NewSession returned nil")
	}
	if session.clientConn != conn {
		t.Error("clientConn not set correctly")
	}
}

func TestSession_Handle_ConnectionError(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:              "invalid-host",
			Port:              12345,
			ConnectionTimeout: 100 * time.Millisecond,
		},
	}
	conn := NewMockConn()
	session := NewSession(conn, cfg, nil)

	err := session.Handle()
	if err == nil {
		t.Error("Expected error when backend connection fails")
	}
}
