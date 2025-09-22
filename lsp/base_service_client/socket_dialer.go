package base

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"
)


type SocketDialer struct {
	network string // "tcp", "unix", "pipe"
	address string // address to connect to
	timeout time.Duration
	conn    net.Conn
}

// NewSocketDialer creates a new socket dialer for the specified network and address.
func NewSocketDialer(network, address string, timeout time.Duration) (*SocketDialer, error) {
	if network == "" {
		return nil, fmt.Errorf("network cannot be empty")
	}
	if address == "" {
		return nil, fmt.Errorf("address cannot be empty")
	}
		
	switch network {
	case "tcp", "unix","pipe":
		// Valid network types
	default:
		return nil, fmt.Errorf("unsupported network type: %s", network)
	}
	
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	
	return &SocketDialer{
		network: network,
		address: address,
		timeout: timeout,
	}, nil
}

// Dial establishes a connection to the LSP server via socket
func (d *SocketDialer) Dial(ctx context.Context) (io.ReadWriteCloser, error) {
	if d.conn != nil {
		return nil, fmt.Errorf("dialer already connected")
	}
	
	var conn net.Conn
	var err error

	dialCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	
	switch d.network {
	case "pipe":
		// Windows named pipe support
		conn, err = d.dialWindowsPipe(dialCtx)
	default:
		dialer := &net.Dialer{}
		conn, err = dialer.DialContext(dialCtx, d.network, d.address)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s://%s: %w", d.network, d.address, err)
	}
	
	d.conn = conn
	return d, nil
}


func (d *SocketDialer) Read(p []byte) (int, error) {
	if d.conn == nil {
		return 0, fmt.Errorf("not connected")
	}
	return d.conn.Read(p)
}


func (d *SocketDialer) Write(p []byte) (int, error) {
	if d.conn == nil {
		return 0, fmt.Errorf("not connected")
	}
	return d.conn.Write(p)
}

func (d *SocketDialer) Close() error {
	if d.conn != nil {
		err := d.conn.Close()
		d.conn = nil
		return err
	}
	return nil
}


func (d *SocketDialer) GetLocalAddr() net.Addr {
	if d.conn != nil {
		return d.conn.LocalAddr()
	}
	return nil
}


func (d *SocketDialer) GetRemoteAddr() net.Addr {
	if d.conn != nil {
		return d.conn.RemoteAddr()
	}
	return nil
}


func (d *SocketDialer) SetDeadline(t time.Time) error {
	if d.conn != nil {
		return d.conn.SetDeadline(t)
	}
	return fmt.Errorf("not connected")
}