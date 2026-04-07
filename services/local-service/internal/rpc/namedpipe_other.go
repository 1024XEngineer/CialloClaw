//go:build !windows

package rpc

import (
	"context"
	"errors"
	"net"
)

var errNamedPipeUnsupported = errors.New("named pipe transport unsupported")

func serveNamedPipe(ctx context.Context, pipeName string, handler func(net.Conn)) error {
	_ = ctx
	_ = pipeName
	_ = handler
	return errNamedPipeUnsupported
}
