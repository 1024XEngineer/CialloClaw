package rpc

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"
)

// Start serves configured transports until one fails or ctx is canceled.
// Shutdown always runs before Start returns, and the supervisor waits for
// transport goroutines so callers do not inherit a partially stopped server.
func (s *Server) Start(ctx context.Context) error {
	supervisor := newTransportSupervisor(s.beginServeRun(ctx), 2)
	defer s.clearServeRun()

	if s.debugHTTPServer != nil {
		supervisor.Go(func(context.Context) error {
			err := s.debugHTTPServer.ListenAndServe()
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return err
		})
	}

	if s.transport == "named_pipe" {
		supervisor.Go(func(ctx context.Context) error {
			err := s.serveNamedPipeWithShutdown(ctx)
			if errors.Is(err, errNamedPipeUnsupported) || ctx.Err() != nil {
				return nil
			}
			return err
		})
	}

	return supervisor.Wait(func() error {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.Shutdown(shutdownCtx)
	})
}

// Shutdown gracefully closes the debug HTTP server and terminates active stream
// handlers so Start does not hand a half-stopped transport back to callers.
func (s *Server) Shutdown(ctx context.Context) error {
	var shutdownErr error

	if s.debugHTTPServer == nil {
	} else if err := s.debugHTTPServer.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		shutdownErr = err
	}

	runCancel, namedPipeCancel, conns := s.beginTransportShutdown()
	if runCancel != nil {
		runCancel()
	}
	if namedPipeCancel != nil {
		namedPipeCancel()
	}
	for _, conn := range conns {
		_ = conn.Close()
	}

	done := make(chan struct{})
	go func() {
		s.streamWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		return shutdownErr
	case <-ctx.Done():
		if shutdownErr != nil {
			return shutdownErr
		}
		return ctx.Err()
	}
}

// transportSupervisor owns the per-Start run context and joins all transport
// workers before the caller regains control.
type transportSupervisor struct {
	ctx    context.Context
	cancel context.CancelFunc
	errCh  chan error
	wg     sync.WaitGroup
}

func newTransportSupervisor(parent context.Context, transports int) *transportSupervisor {
	if transports < 1 {
		transports = 1
	}
	ctx, cancel := context.WithCancel(parent)
	return &transportSupervisor{
		ctx:    ctx,
		cancel: cancel,
		errCh:  make(chan error, transports),
	}
}

func (s *transportSupervisor) Go(run func(context.Context) error) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := run(s.ctx); err != nil {
			select {
			case s.errCh <- err:
			case <-s.ctx.Done():
			}
		}
	}()
}

// Wait returns the first transport error after canceling sibling transports,
// running shutdown, and joining every started transport goroutine.
func (s *transportSupervisor) Wait(shutdown func() error) error {
	var transportErr error
	select {
	case transportErr = <-s.errCh:
	case <-s.ctx.Done():
	}

	s.cancel()
	shutdownErr := shutdown()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	<-done

	if transportErr != nil {
		return transportErr
	}
	return shutdownErr
}

func (s *Server) beginServeRun(parent context.Context) context.Context {
	runCtx, runCancel := context.WithCancel(parent)

	s.streamMu.Lock()
	s.runCancel = runCancel
	s.namedPipeCancel = nil
	s.shuttingDown = false
	s.streamMu.Unlock()

	return runCtx
}

func (s *Server) clearServeRun() {
	s.streamMu.Lock()
	s.runCancel = nil
	s.namedPipeCancel = nil
	s.streamMu.Unlock()
}

func (s *Server) serveNamedPipeWithShutdown(parent context.Context) error {
	listenerCtx, listenerCancel := context.WithCancel(parent)

	s.streamMu.Lock()
	s.namedPipeCancel = listenerCancel
	s.streamMu.Unlock()

	defer func() {
		s.streamMu.Lock()
		s.namedPipeCancel = nil
		s.streamMu.Unlock()
	}()

	return s.serveNamedPipe(listenerCtx, s.namedPipeName, s.handleStreamConn)
}

func (s *Server) beginTransportShutdown() (context.CancelFunc, context.CancelFunc, []net.Conn) {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()

	s.shuttingDown = true
	runCancel := s.runCancel
	namedPipeCancel := s.namedPipeCancel
	s.runCancel = nil
	s.namedPipeCancel = nil
	conns := make([]net.Conn, 0, len(s.streamConns))
	for conn := range s.streamConns {
		conns = append(conns, conn)
	}
	return runCancel, namedPipeCancel, conns
}
