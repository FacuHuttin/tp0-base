package common

import (
	"net"
	"time"
)

type Connection interface {
	Connect() error
	ConnectWithTimeout(timeout time.Duration) error
	ReceiveExactBytes(count int) ([]byte, error)
	Send(message []byte) error
	Close() error
}

type TCPConnection struct {
	conn    net.Conn
	address string
}

func NewTCPConnection(address string) *TCPConnection {
	return &TCPConnection{address: address}
}

func (t *TCPConnection) Connect() error {
	if t.conn != nil {
		return nil // Already connected
	}
	conn, err := net.Dial("tcp", t.address)
	if err != nil {
		return err
	}
	t.conn = conn
	return nil
}

func (t *TCPConnection) ConnectWithTimeout(timeout time.Duration) error {
	if t.conn != nil {
		return nil // Already connected
	}
	conn, err := net.DialTimeout("tcp", t.address, timeout)
	if err != nil {
		return err
	}
	t.conn = conn
	return nil
}

func (t *TCPConnection) Send(message []byte) error {
	totalWritten := 0
	for totalWritten < len(message) {
		written, err := t.conn.Write(message[totalWritten:])
		if err != nil {
			return err
		}
		totalWritten += written
	}
	return nil
}

func (t *TCPConnection) ReceiveExactBytes(count int) ([]byte, error) {
	data := make([]byte, count)
	totalRead := 0

	for totalRead < count {
		n, err := t.conn.Read(data[totalRead:])
		if err != nil {
			return nil, err
		}
		totalRead += n
	}

	return data, nil
}

func (t *TCPConnection) Close() error {
	if t.conn != nil {
		err := t.conn.Close()
		t.conn = nil // Reset connection
		return err
	}
	return nil
}
