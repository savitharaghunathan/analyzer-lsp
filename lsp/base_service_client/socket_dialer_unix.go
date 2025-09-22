//go:build !windows

package base

import (
	"context"
	"fmt"
	"net"
)


func (d *SocketDialer) dialWindowsPipe(ctx context.Context) (net.Conn, error) {
	return nil, fmt.Errorf("named pipes are not supported on this platform")
}