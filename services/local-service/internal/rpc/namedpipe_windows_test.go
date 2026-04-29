//go:build windows

package rpc

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	winio "github.com/Microsoft/go-winio"
)

func TestServeNamedPipeAcceptsConnectionAndStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pipeName := fmt.Sprintf(`\\.\pipe\cialloclaw-rpc-test-%d`, time.Now().UnixNano())
	handled := make(chan struct{}, 1)
	errCh := make(chan error, 1)

	go func() {
		errCh <- serveNamedPipe(ctx, pipeName, func(conn net.Conn) {
			_ = conn.Close()
			select {
			case handled <- struct{}{}:
			default:
			}
		})
	}()

	timeout := 2 * time.Second
	var conn net.Conn
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		currentConn, err := winio.DialPipe(pipeName, &timeout)
		if err == nil {
			conn = currentConn
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if conn == nil {
		t.Fatal("expected named pipe listener to accept a client connection")
	}
	defer conn.Close()

	select {
	case <-handled:
	case <-time.After(2 * time.Second):
		t.Fatal("expected named pipe handler to run")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected named pipe listener to exit cleanly on cancel, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected named pipe listener to stop after cancel")
	}
}
