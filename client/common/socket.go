package common

import (
	"net"
	"time"
)

// TCPConn is a concrete implementation of TCPConnection using net.Conn.
type TCPConn struct {
	conn net.Conn
}

// NewTCPConn creates a new TCPConn.
func NewTCPConn(address string) (*TCPConn,error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	return &TCPConn{conn: conn}, nil
}

// ReadFull reads exactly len(b) bytes from the connection.
func (t *TCPConn) ReadFull(b []byte) (n int, err error) {
	totalRead := 0
	for totalRead < len(b) {
		n, err := t.conn.Read(b[totalRead:])
		if err != nil {
			return totalRead, err
		}
		totalRead += n
	}
	return totalRead, nil
}

// WriteFull writes exactly len(b) bytes to the connection.
func (t *TCPConn) WriteFull(b []byte) (n int, err error) {
	totalWritten := 0
	for totalWritten < len(b) {
		n, err := t.conn.Write(b[totalWritten:])
		if err != nil {
			return totalWritten, err
		}
		totalWritten += n
	}
	return totalWritten, nil
}

// Close closes the connection.
func (t *TCPConn) Close() error {
	return t.conn.Close()
}

// SetDeadline sets the read and write deadlines associated with the connection.
func (t *TCPConn) SetDeadline(time time.Time) error {
	return t.conn.SetDeadline(time)
}

// SetReadDeadline sets the deadline for future Read calls.
func (t *TCPConn) SetReadDeadline(time time.Time) error {
	return t.conn.SetReadDeadline(time)
}

// SetWriteDeadline sets the deadline for future Write calls.
func (t *TCPConn) SetWriteDeadline(time time.Time) error {
	return t.conn.SetWriteDeadline(time)
}