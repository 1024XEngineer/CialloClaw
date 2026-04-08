//go:build windows

// 该文件负责 Windows 下的 Named Pipe 监听骨架。
package rpc

import (
	"context"
	"errors"
	"net"

	winio "github.com/Microsoft/go-winio"
)

var errNamedPipeUnsupported = errors.New("named pipe transport unsupported")

func serveNamedPipe(ctx context.Context, pipeName string, handler func(net.Conn)) error {
	listener, err := winio.ListenPipe(pipeName, &winio.PipeConfig{
		MessageMode:      true,
		InputBufferSize:  64 * 1024,
		OutputBufferSize: 64 * 1024,
	})
	if err != nil {
		return err
	}

	defer listener.Close()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}

		go handler(conn)
	}
}
