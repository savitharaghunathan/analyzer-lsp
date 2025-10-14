package rpc

import (
	"context"

	jsonrpc2 "github.com/konveyor/analyzer-lsp/jsonrpc2_v2"
	"github.com/konveyor/analyzer-lsp/provider"
)

// ClientWrapper wraps a jsonrpc2_v2 Connection to implement the provider.RPCClient interface
// This bridges the gap between jsonrpc2_v2's async Call method and the synchronous interface expected by providers
type ClientWrapper struct {
	conn *jsonrpc2.Connection
}

// NewClientWrapper creates a new wrapper around a jsonrpc2_v2 Connection
func NewClientWrapper(conn *jsonrpc2.Connection) provider.RPCClient {
	return &ClientWrapper{
		conn: conn,
	}
}

// Call implements provider.RPCClient.Call by wrapping the async jsonrpc2_v2 Call with Await
func (r *ClientWrapper) Call(ctx context.Context, method string, args, result interface{}) error {
	return r.conn.Call(ctx, method, args).Await(ctx, result)
}

// Notify implements provider.RPCClient.Notify by delegating to the jsonrpc2_v2 Connection
func (r *ClientWrapper) Notify(ctx context.Context, method string, args interface{}) error {
	return r.conn.Notify(ctx, method, args)
}

// Close implements provider.RPCClient.Close by delegating to the jsonrpc2_v2 Connection
func (r *ClientWrapper) Close() error {
	return r.conn.Close()
}