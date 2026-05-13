package rpc

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"
)

const defaultTransportShutdownTimeout = 5 * time.Minute

var (
	errTransportShutdownIncomplete = errors.New("transport shutdown incomplete")
	errServerAlreadyStarted        = errors.New("server already started")
)

// Start serves configured transports until one fails or ctx is canceled.
// Shutdown always runs before Start returns, and the supervisor waits for
// transport goroutines so callers do not inherit a partially stopped server.
func (s *Server) Start(ctx context.Context) error {
	s.streamMu.Lock()
	if s.runCtx != nil {
		s.streamMu.Unlock()
		return errServerAlreadyStarted
	}
	runCtx, runCancel := context.WithCancelCause(ctx)
	s.runCtx = runCtx
	s.runCancel = runCancel
	s.shuttingDown = false
	s.streamMu.Unlock()
	defer func() {
		s.streamMu.Lock()
		if s.runCtx == runCtx {
			s.runCancel = nil
		}
		s.streamMu.Unlock()
	}()

	supervisor := newTransportSupervisor(runCtx, 2)

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
		serveNamedPipeFn := s.serveNamedPipe
		if serveNamedPipeFn == nil {
			serveNamedPipeFn = serveNamedPipe
		}
		supervisor.Go(func(ctx context.Context) error {
			err := serveNamedPipeFn(ctx, s.namedPipeName, s.handleStreamConn)
			if errors.Is(err, errNamedPipeUnsupported) || ctx.Err() != nil {
				return nil
			}
			return err
		})
	}

	err := supervisor.Wait(s.transportShutdownTimeout, func(shutdownCtx context.Context) error {
		return s.Shutdown(shutdownCtx)
	})
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.Join(errTransportShutdownIncomplete, err)
	}
	return err
}

// Shutdown gracefully closes the debug HTTP server and terminates active stream
// handlers so Start does not hand a half-stopped transport back to callers.
func (s *Server) Shutdown(ctx context.Context) error {
	s.streamMu.Lock()
	runCancel := s.runCancel
	s.streamMu.Unlock()
	if runCancel != nil {
		runCancel(nil)
	}

	var shutdownErr error

	conns := s.beginStreamShutdown()
	for _, conn := range conns {
		_ = conn.Close()
	}

	if s.debugHTTPServer == nil {
	} else if err := s.debugHTTPServer.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		shutdownErr = err
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
func (s *transportSupervisor) Wait(timeout time.Duration, shutdown func(context.Context) error) error {
	if timeout <= 0 {
		timeout = defaultTransportShutdownTimeout
	}

	var transportErr error
	select {
	case transportErr = <-s.errCh:
	case <-s.ctx.Done():
	}

	s.cancel()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	shutdownErr := shutdown(shutdownCtx)

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-shutdownCtx.Done():
		return shutdownCtx.Err()
	}

	if transportErr != nil {
		return transportErr
	}
	return shutdownErr
}

func (s *Server) beginStreamShutdown() []net.Conn {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()

	s.shuttingDown = true
	conns := make([]net.Conn, 0, len(s.streamConns))
	for conn := range s.streamConns {
		conns = append(conns, conn)
	}
	return conns
}
