package rpc

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"
)

// Start serves configured transports until one fails or ctx is canceled.
// Shutdown always runs before Start returns, and the supervisor waits for
// transport goroutines so callers do not inherit a partially stopped server.
func (s *Server) Start(ctx context.Context) error {
	supervisor := newTransportSupervisor(ctx, 2)

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
			err := serveNamedPipe(ctx, s.namedPipeName, s.handleStreamConn)
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

// Shutdown gracefully closes the debug HTTP server when it was configured.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.debugHTTPServer == nil {
		return nil
	}

	if err := s.debugHTTPServer.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
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
