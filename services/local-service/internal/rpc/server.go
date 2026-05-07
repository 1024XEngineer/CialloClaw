// Package rpc hosts the local JSON-RPC server and debug transports.
package rpc

import (
	"context"
	"errors"
	"net/http"
	"time"

	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/orchestrator"
)

// Server is the transport entrypoint for local-service.
// It accepts debug HTTP, named-pipe streams, and dispatches stable JSON-RPC
// methods into the orchestrator.
type Server struct {
	transport       string
	namedPipeName   string
	debugHTTPServer *http.Server
	handlers        map[string]methodHandler
	orchestrator    *orchestrator.Service
	now             func() time.Time
}

// NewServer constructs the RPC server and registers debug endpoints.
func NewServer(cfg serviceconfig.RPCConfig, orchestrator *orchestrator.Service) *Server {
	server := &Server{
		transport:     cfg.Transport,
		namedPipeName: cfg.NamedPipeName,
		orchestrator:  orchestrator,
		now:           time.Now,
	}

	server.registerHandlers()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", server.handleHealthz)
	mux.HandleFunc("/rpc", server.handleHTTPRPC)
	mux.HandleFunc("/events", server.handleDebugEvents)
	mux.HandleFunc("/events/stream", server.handleDebugEventStream)

	server.debugHTTPServer = &http.Server{
		Addr:              cfg.DebugHTTPAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return server
}

// Start serves every transport enabled by the current config.
// During P0 it intentionally keeps both debug HTTP and named pipe available for
// local integration work.
func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 2)

	if s.debugHTTPServer != nil {
		go func() {
			err := s.debugHTTPServer.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
		}()
	}

	if s.transport == "named_pipe" {
		go func() {
			err := serveNamedPipe(ctx, s.namedPipeName, s.handleStreamConn)
			if err != nil && !errors.Is(err, errNamedPipeUnsupported) && ctx.Err() == nil {
				errCh <- err
			}
		}()
	}

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.Shutdown(shutdownCtx)
	}
}

// Shutdown closes the debug HTTP server when it was started.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.debugHTTPServer == nil {
		return nil
	}

	if err := s.debugHTTPServer.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

// dispatch is the single RPC dispatch path that validates protocol shape,
// resolves handlers, decodes params, and rewraps orchestrator output.
func (s *Server) dispatch(request requestEnvelope) any {
	if request.JSONRPC != "2.0" {
		return newErrorEnvelope(request.ID, &rpcError{
			Code:    errInvalidParams,
			Message: "INVALID_PARAMS",
			Detail:  "jsonrpc version must be 2.0",
			TraceID: "trace_rpc_version",
		})
	}

	handler, ok := s.handlers[request.Method]
	if !ok {
		return newErrorEnvelope(request.ID, &rpcError{
			Code:    errMethodNotFound,
			Message: "JSON_RPC_METHOD_NOT_FOUND",
			Detail:  "method is not registered in the stable stub router",
			TraceID: traceIDFromRequest(request.Params),
		})
	}

	params, rpcErr := decodeParams(request.Params)
	if rpcErr != nil {
		return newErrorEnvelope(request.ID, rpcErr)
	}

	data, handlerErr := handler(params)
	if handlerErr != nil {
		return newErrorEnvelope(request.ID, handlerErr)
	}

	return newSuccessEnvelope(request.ID, data, s.nowRFC3339())
}

// nowRFC3339 returns the unified response timestamp format.
func (s *Server) nowRFC3339() string {
	return s.now().Format(time.RFC3339)
}
