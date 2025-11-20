package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Packet represents a MySQL wire protocol packet
type Packet struct {
	Length     uint32
	SequenceID uint8
	Payload    []byte
}

// ReadPacket reads a packet from the connection
func ReadPacket(r io.Reader) (*Packet, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}

	length := uint32(header[0]) | uint32(header[1])<<8 | uint32(header[2])<<16
	sequenceID := header[3]

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}

	return &Packet{
		Length:     length,
		SequenceID: sequenceID,
		Payload:    payload,
	}, nil
}

// WritePacket writes a packet to the connection
func WritePacket(w io.Writer, sequenceID uint8, payload []byte) error {
	length := len(payload)
	if length > 16777215 {
		return fmt.Errorf("packet too large: %d", length)
	}

	header := make([]byte, 4)
	header[0] = byte(length)
	header[1] = byte(length >> 8)
	header[2] = byte(length >> 16)
	header[3] = sequenceID

	if _, err := w.Write(header); err != nil {
		return err
	}

	if _, err := w.Write(payload); err != nil {
		return err
	}

	return nil
}

// WriteString writes a length-encoded string
func WriteString(buf []byte, s string) []byte {
	buf = append(buf, s...)
	buf = append(buf, 0x00)
	return buf
}

// WriteUint16 writes a uint16
func WriteUint16(buf []byte, n uint16) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, n)
	return append(buf, b...)
}

// WriteUint32 writes a uint32
func WriteUint32(buf []byte, n uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, n)
	return append(buf, b...)
}
