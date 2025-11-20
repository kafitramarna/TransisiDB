package protocol

import (
	"encoding/binary"
	"fmt"
)

// OKPacket represents a MySQL OK packet
type OKPacket struct {
	AffectedRows uint64
	LastInsertID uint64
	StatusFlags  uint16
	Warnings     uint16
	Info         string
}

// ERRPacket represents a MySQL error packet
type ERRPacket struct {
	ErrorCode    uint16
	SQLState     string
	ErrorMessage string
}

// EOFPacket represents a MySQL EOF packet
type EOFPacket struct {
	Warnings    uint16
	StatusFlags uint16
}

// ParseOKPacket parses an OK packet payload
func ParseOKPacket(payload []byte) (*OKPacket, error) {
	if len(payload) < 7 {
		return nil, fmt.Errorf("OK packet too short: %d bytes", len(payload))
	}

	if payload[0] != OK_PACKET {
		return nil, fmt.Errorf("not an OK packet: 0x%02X", payload[0])
	}

	pos := 1
	affectedRows, n := readLengthEncodedInt(payload[pos:])
	pos += n

	lastInsertID, n := readLengthEncodedInt(payload[pos:])
	pos += n

	if pos+4 > len(payload) {
		return nil, fmt.Errorf("unexpected end of OK packet")
	}

	statusFlags := binary.LittleEndian.Uint16(payload[pos:])
	pos += 2

	warnings := binary.LittleEndian.Uint16(payload[pos:])
	pos += 2

	var info string
	if pos < len(payload) {
		info = string(payload[pos:])
	}

	return &OKPacket{
		AffectedRows: affectedRows,
		LastInsertID: lastInsertID,
		StatusFlags:  statusFlags,
		Warnings:     warnings,
		Info:         info,
	}, nil
}

// ParseERRPacket parses an error packet payload
func ParseERRPacket(payload []byte) (*ERRPacket, error) {
	if len(payload) < 9 {
		return nil, fmt.Errorf("ERR packet too short: %d bytes", len(payload))
	}

	if payload[0] != ERR_PACKET {
		return nil, fmt.Errorf("not an ERR packet: 0x%02X", payload[0])
	}

	errorCode := binary.LittleEndian.Uint16(payload[1:3])

	pos := 3
	var sqlState string
	var errorMessage string

	// Check for SQL state marker (#)
	if payload[pos] == '#' {
		sqlState = string(payload[pos+1 : pos+6])
		pos += 6
	}

	errorMessage = string(payload[pos:])

	return &ERRPacket{
		ErrorCode:    errorCode,
		SQLState:     sqlState,
		ErrorMessage: errorMessage,
	}, nil
}

// ParseEOFPacket parses an EOF packet payload
func ParseEOFPacket(payload []byte) (*EOFPacket, error) {
	if len(payload) < 5 {
		return nil, fmt.Errorf("EOF packet too short: %d bytes", len(payload))
	}

	if payload[0] != EOF_PACKET {
		return nil, fmt.Errorf("not an EOF packet: 0x%02X", payload[0])
	}

	warnings := binary.LittleEndian.Uint16(payload[1:3])
	statusFlags := binary.LittleEndian.Uint16(payload[3:5])

	return &EOFPacket{
		Warnings:    warnings,
		StatusFlags: statusFlags,
	}, nil
}

// IsOKPacket checks if a payload is an OK packet
func IsOKPacket(payload []byte) bool {
	if len(payload) < 7 {
		return false
	}
	// OK packet: 0x00 and length < 0xFFFFFF
	return payload[0] == OK_PACKET
}

// IsEOFPacket checks if a payload is an EOF packet
func IsEOFPacket(payload []byte) bool {
	if len(payload) < 5 {
		return false
	}
	// EOF packet: 0xFE and length < 9
	return payload[0] == EOF_PACKET && len(payload) < 9
}

// IsERRPacket checks if a payload is an ERR packet
func IsERRPacket(payload []byte) bool {
	if len(payload) < 3 {
		return false
	}
	return payload[0] == ERR_PACKET
}

// readLengthEncodedInt reads a MySQL length-encoded integer
func readLengthEncodedInt(b []byte) (uint64, int) {
	if len(b) == 0 {
		return 0, 0
	}

	switch b[0] {
	case 0xfb:
		return 0, 1 // NULL
	case 0xfc:
		// 2-byte int
		if len(b) < 3 {
			return 0, 0
		}
		return uint64(binary.LittleEndian.Uint16(b[1:3])), 3
	case 0xfd:
		// 3-byte int
		if len(b) < 4 {
			return 0, 0
		}
		return uint64(b[1]) | uint64(b[2])<<8 | uint64(b[3])<<16, 4
	case 0xfe:
		// 8-byte int
		if len(b) < 9 {
			return 0, 0
		}
		return binary.LittleEndian.Uint64(b[1:9]), 9
	default:
		// 1-byte int
		return uint64(b[0]), 1
	}
}

// readLengthEncodedString reads a MySQL length-encoded string
func readLengthEncodedString(b []byte) (string, int, error) {
	length, n := readLengthEncodedInt(b)
	if n == 0 {
		return "", 0, fmt.Errorf("failed to read length")
	}

	if length == 0 {
		return "", n, nil
	}

	if uint64(len(b)) < uint64(n)+length {
		return "", 0, fmt.Errorf("not enough data for string")
	}

	return string(b[n : uint64(n)+length]), n + int(length), nil
}

// WriteLengthEncodedInt writes a length-encoded integer
func WriteLengthEncodedInt(buf []byte, n uint64) []byte {
	if n < 251 {
		return append(buf, byte(n))
	} else if n < 1<<16 {
		buf = append(buf, 0xfc)
		b := make([]byte, 2)
		binary.LittleEndian.PutUint16(b, uint16(n))
		return append(buf, b...)
	} else if n < 1<<24 {
		buf = append(buf, 0xfd)
		buf = append(buf, byte(n), byte(n>>8), byte(n>>16))
		return buf
	} else {
		buf = append(buf, 0xfe)
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, n)
		return append(buf, b...)
	}
}

// WriteLengthEncodedString writes a length-encoded string
func WriteLengthEncodedString(buf []byte, s string) []byte {
	buf = WriteLengthEncodedInt(buf, uint64(len(s)))
	return append(buf, []byte(s)...)
}
