package peer

import (
	"fmt"
	"net"
)

// handleChoke ...
func handleChoke(conn net.Conn) error {
	return fmt.Errorf("not implemented")
}

// handleUnchoke ...
func handleUnchoke(conn net.Conn) error {
	return fmt.Errorf("not implemented")
}

// handleInterested ...
func handleInterested(conn net.Conn) error {
	return fmt.Errorf("not implemented")
}

// handleChoke ...
func handleNotInterested(conn net.Conn) error {
	return fmt.Errorf("not implemented")
}

// handleHave ...
func handleHave(conn net.Conn) error {
	return fmt.Errorf("not implemented")
}

// handleBitfield ...
func handleBitfield(conn net.Conn) error {
	return fmt.Errorf("not implemented")
}

// handleRequest ...
func handleRequest(conn net.Conn) error {
	return fmt.Errorf("not implemented")
}

// handlePiece ...
func handlePiece(conn net.Conn) error {
	return fmt.Errorf("not implemented")
}

// handleCancel...
func handleCancel(conn net.Conn) error {
	return fmt.Errorf("not implemented")
}

// handlePort ...
func handlePort(conn net.Conn) error {
	return fmt.Errorf("not implemented")
}
