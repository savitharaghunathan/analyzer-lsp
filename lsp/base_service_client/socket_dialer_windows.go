//go:build windows

package base

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

// WindowsPipeDialer handles Windows named pipe connections
type WindowsPipeDialer struct {
	pipeName string
	timeout  time.Duration
	conn     net.Conn
}

// NewWindowsPipeDialer creates a dialer for Windows named pipes
func NewWindowsPipeDialer(address string, timeout time.Duration) (*WindowsPipeDialer, error) {
	pipeName := address
	
	// Normalize pipe name format
	if !strings.HasPrefix(pipeName, `\\.\pipe\`) {
		if strings.HasPrefix(pipeName, `pipe\`) {
			pipeName = `\\.\` + pipeName
		} else {
			pipeName = `\\.\pipe\` + pipeName
		}
	}
	
	return &WindowsPipeDialer{
		pipeName: pipeName,
		timeout:  timeout,
	}, nil
}

// Dial connects to the Windows named pipe
func (d *WindowsPipeDialer) Dial(ctx context.Context) (net.Conn, error) {
	if d.conn != nil {
		return nil, fmt.Errorf("pipe dialer already connected")
	}
	
	// Create a context with timeout
	dialCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	
	dialer := &net.Dialer{}
	
	
	conn, err := dialer.DialContext(dialCtx) 
	if err != nil {
		return nil, fmt.Errorf("failed to connect to named pipe %s: %w", d.pipeName, err)
	}
	
	d.conn = conn
	return conn, nil
}

// dialWindowsPipe handles Windows named pipe connections
func (d *SocketDialer) dialWindowsPipe(ctx context.Context) (net.Conn, error) {
	pipeDialer, err := NewWindowsPipeDialer(d.address, d.timeout)
	if err != nil {
		return nil, err
	}
	
	return pipeDialer.Dial(ctx)
}