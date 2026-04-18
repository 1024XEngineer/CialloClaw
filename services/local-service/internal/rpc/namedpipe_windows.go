//go:build windows

// Package rpc provides the Windows named-pipe listener used by the local RPC
// server.
package rpc

import (
	"context"
	"errors"
	"net"

	winio "github.com/Microsoft/go-winio"
)

// errNamedPipeUnsupported is kept for parity with other transports.
var errNamedPipeUnsupported = errors.New("named pipe transport unsupported")

// serveNamedPipe accepts Windows named-pipe connections and hands each stream
// to the server handler.
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
