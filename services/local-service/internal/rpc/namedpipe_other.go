//go:build !windows

// Package rpc provides the non-Windows named-pipe placeholder for the local
// RPC server.
package rpc

import (
	"context"
	"errors"
	"net"
)

// errNamedPipeUnsupported documents that named-pipe transport is unavailable on
// non-Windows platforms.
var errNamedPipeUnsupported = errors.New("named pipe transport unsupported")

// serveNamedPipe reports that named-pipe transport is not supported here.
func serveNamedPipe(ctx context.Context, pipeName string, handler func(net.Conn)) error {
	_ = ctx
	_ = pipeName
	_ = handler
	return errNamedPipeUnsupported
}
