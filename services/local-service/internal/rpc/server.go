package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/orchestrator"
)

type Server struct {
	transport       string
	namedPipeName   string
	debugHTTPServer *http.Server
	handlers        map[string]methodHandler
	orchestrator    *orchestrator.Service
	now             func() time.Time
}

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

	server.debugHTTPServer = &http.Server{
		Addr:              cfg.DebugHTTPAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return server
}

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

func (s *Server) Shutdown(ctx context.Context) error {
	if s.debugHTTPServer == nil {
		return nil
	}

	if err := s.debugHTTPServer.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":       "ok",
		"service":      "local-service",
		"transport":    s.transport,
		"named_pipe":   s.namedPipeName,
		"orchestrator": s.orchestrator.Snapshot(),
	})
}

func (s *Server) handleHTTPRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	request, rpcErr := decodeRequest(r.Body)
	if rpcErr != nil {
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(newErrorEnvelope(nil, rpcErr))
		return
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(s.dispatch(request))
}

func (s *Server) handleStreamConn(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {
		var request requestEnvelope
		if err := decoder.Decode(&request); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}

			_ = encoder.Encode(newErrorEnvelope(nil, &rpcError{
				Code:    errInvalidParams,
				Message: "INVALID_PARAMS",
				Detail:  "invalid json-rpc payload",
				TraceID: "trace_rpc_decode",
			}))
			return
		}

		if err := encoder.Encode(s.dispatch(request)); err != nil {
			return
		}
	}
}

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
