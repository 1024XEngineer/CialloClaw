//go:build !windows

// 该文件负责非 Windows 环境下的 Named Pipe 占位实现。
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
