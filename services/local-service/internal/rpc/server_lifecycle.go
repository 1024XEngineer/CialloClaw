package rpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

const defaultTransportShutdownTimeout = 5 * time.Second

var errTransportShutdownIncomplete = errors.New("rpc transport shutdown incomplete")

// Start serves configured transports until one fails or ctx is canceled.
// Shutdown always runs before Start returns; if a transport misses the shutdown
// window, the server is fenced into a terminal state instead of being reusable.
func (s *Server) Start(ctx context.Context) error {
	runCtx, err := s.beginServeRun(ctx)
	if err != nil {
		return err
	}

	supervisor := newTransportSupervisor(runCtx, 2)
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

	if err := supervisor.Wait(s.shutdownTimeout(), s.Shutdown); err != nil {
		if isTransportShutdownIncomplete(err) {
			return s.markTransportTerminal(err)
		}
		return err
	}
	return nil
}

// Shutdown gracefully closes the debug HTTP server and terminates active stream
// handlers. Any incomplete stop fences this Server instance from future reuse.
func (s *Server) Shutdown(ctx context.Context) error {
	var shutdownErr error

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

	if s.debugHTTPServer != nil {
		if err := s.debugHTTPServer.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			shutdownErr = err
		}
	}

	if err := waitWithContext(ctx, s.streamWG.Wait); err != nil {
		if shutdownErr != nil {
			return s.markTransportTerminal(shutdownErr)
		}
		return s.markTransportTerminal(err)
	}

	if shutdownErr != nil {
		return s.markTransportTerminal(shutdownErr)
	}
	return nil
}

// transportSupervisor owns the per-Start run context and coordinates transport
// cancellation, graceful shutdown, and bounded worker joins.
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

// Wait returns the first transport error after canceling sibling transports and
// running shutdown. A worker that misses the bounded shutdown window returns the
// context error so callers can fence the owning server before reuse.
func (s *transportSupervisor) Wait(timeout time.Duration, shutdown func(context.Context) error) error {
	var transportErr error
	select {
	case transportErr = <-s.errCh:
	case <-s.ctx.Done():
	}

	s.cancel()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	shutdownErr := shutdown(shutdownCtx)
	waitErr := waitWithContext(shutdownCtx, s.wg.Wait)

	if waitErr != nil {
		return waitErr
	}
	if shutdownErr != nil {
		return shutdownErr
	}
	if transportErr != nil {
		return transportErr
	}
	return nil
}

func (s *Server) beginServeRun(parent context.Context) (context.Context, error) {
	runCtx, runCancel := context.WithCancel(parent)

	s.streamMu.Lock()
	defer s.streamMu.Unlock()

	if s.terminalErr != nil {
		runCancel()
		return nil, s.terminalErr
	}

	s.runCancel = runCancel
	s.namedPipeCancel = nil
	s.shuttingDown = false
	return runCtx, nil
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

func (s *Server) shutdownTimeout() time.Duration {
	if s.transportShutdownTimeout > 0 {
		return s.transportShutdownTimeout
	}
	return defaultTransportShutdownTimeout
}

func (s *Server) markTransportTerminal(cause error) error {
	if cause == nil {
		return nil
	}

	terminalErr := cause
	if !errors.Is(cause, errTransportShutdownIncomplete) {
		terminalErr = fmt.Errorf("%w: %w", errTransportShutdownIncomplete, cause)
	}

	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	if s.terminalErr == nil {
		s.terminalErr = terminalErr
	}
	return s.terminalErr
}

func isTransportShutdownIncomplete(err error) bool {
	return errors.Is(err, errTransportShutdownIncomplete) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}

func waitWithContext(ctx context.Context, wait func()) error {
	done := make(chan struct{})
	go func() {
		wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
