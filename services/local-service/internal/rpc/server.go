// Package rpc hosts the local JSON-RPC server and debug transports.
package rpc

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/orchestrator"
)

// Server owns the local-service RPC transports and stable JSON-RPC router.
// Protocol envelopes, handler behavior, and notification routing live in
// dedicated files so transport lifecycle stays easy to audit.
type Server struct {
	transport                string
	namedPipeName            string
	debugHTTPServer          *http.Server
	handlers                 map[string]methodHandler
	orchestrator             *orchestrator.Service
	serveNamedPipe           func(ctx context.Context, pipeName string, handler func(net.Conn)) error
	now                      func() time.Time
	transportShutdownTimeout time.Duration
	streamMu                 sync.Mutex
	streamConns              map[net.Conn]struct{}
	streamWG                 sync.WaitGroup
	serveRunning             bool
	runCancel                context.CancelFunc
	shuttingDown             bool
	terminalErr              error
}

// NewServer wires configured transports to registered handlers without
// starting network or named-pipe listeners.
func NewServer(cfg serviceconfig.RPCConfig, orchestrator *orchestrator.Service) *Server {
	server := &Server{
		transport:                cfg.Transport,
		namedPipeName:            cfg.NamedPipeName,
		orchestrator:             orchestrator,
		serveNamedPipe:           serveNamedPipe,
		now:                      time.Now,
		transportShutdownTimeout: defaultTransportShutdownTimeout,
		streamConns:              make(map[net.Conn]struct{}),
	}

	server.registerHandlers()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", server.handleHealthz)
	mux.HandleFunc("/rpc", server.handleHTTPRPC)
	mux.HandleFunc("/events", server.handleDebugEvents)
	mux.HandleFunc("/events/stream", server.handleDebugEventStream)

	if cfg.DebugHTTPAddress != "" {
		server.debugHTTPServer = &http.Server{
			Addr:              cfg.DebugHTTPAddress,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		}
	}

	return server
}
