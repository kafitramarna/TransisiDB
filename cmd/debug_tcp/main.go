package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

func main() {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:3307", 5*time.Second)
	if err != nil {
		log.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	fmt.Println("Connected to 127.0.0.1:3307")

	// Read first packet (Handshake)
	header := make([]byte, 4)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.ReadFull(conn, header); err != nil {
		log.Fatalf("Read header failed: %v", err)
	}

	length := uint32(header[0]) | uint32(header[1])<<8 | uint32(header[2])<<16
	seq := header[3]

	fmt.Printf("Header read: Length=%d, Seq=%d\n", length, seq)

	payload := make([]byte, length)
	if _, err := io.ReadFull(conn, payload); err != nil {
		log.Fatalf("Read payload failed: %v", err)
	}

	fmt.Printf("Handshake packet received (%d bytes)\n", len(payload))
}
