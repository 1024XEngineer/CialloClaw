package rpc

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"
)

// handleStreamConn serves one long-lived JSON-RPC stream. Live runtime
// notifications can be emitted before the matching response, while buffered
// task notifications are replayed on the same connection after the response.
func (s *Server) handleStreamConn(conn net.Conn) {
	if !s.registerStreamConn(conn) {
		_ = conn.Close()
		return
	}
	defer s.unregisterStreamConn(conn)
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)
	var writeMu sync.Mutex
	writeEnvelope := func(envelope any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return encoder.Encode(envelope)
	}

	for {
		var request requestEnvelope
		if err := decoder.Decode(&request); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}

			_ = writeEnvelope(newErrorEnvelope(nil, &rpcError{
				Code:    errInvalidParams,
				Message: "INVALID_PARAMS",
				Detail:  "invalid json-rpc payload",
				TraceID: "trace_rpc_decode",
			}))
			return
		}

		tracker := newStreamRequestTracker(request)
		unsubscribeRuntime := func() {}
		if tracker.shouldSubscribeRuntime() {
			unsubscribeRuntime = s.orchestrator.SubscribeRuntimeNotifications(func(taskID string, method string, params map[string]any) {
				if !isLiveRuntimeMethod(method) {
					return
				}
				notificationTaskID := runtimeNotificationTaskID(taskID, params)
				if notificationTaskID == "" || !tracker.hasTaskID(notificationTaskID) {
					return
				}
				reservationKey := tracker.reserveStreamedRuntime(method, notificationTaskID, params)
				err := writeEnvelope(newNotificationEnvelope(method, params))
				if err != nil {
					tracker.releaseStreamedRuntimeReservation(reservationKey)
				}
			})
		}

		unsubscribeTaskStart := func() {}
		if tracker.shouldSubscribeTaskStart() {
			unsubscribeTaskStart = s.orchestrator.SubscribeTaskStarts(func(taskID, sessionID, traceID string) {
				if !tracker.matchesTaskStart(sessionID, traceID) {
					return
				}
				tracker.addTaskID(taskID)
			})
		}

		response := s.dispatch(request)
		unsubscribeTaskStart()
		unsubscribeRuntime()

		err := writeEnvelope(response)
		if err != nil {
			return
		}

		for _, taskID := range taskIDsFromResponse(response) {
			notifications, err := s.orchestrator.DrainNotifications(taskID)
			if err != nil {
				continue
			}

			for _, notification := range notifications {
				method := stringValue(notification, "method", "task.updated")
				params := mapValue(notification, "params")
				if tracker.shouldSkipBufferedRuntime(method, taskID, params) {
					continue
				}
				err := writeEnvelope(newNotificationEnvelope(method, params))
				if err != nil {
					return
				}
			}
		}
	}
}

// registerStreamConn binds a named-pipe stream to the current server lifetime.
// Once shutdown starts, new handlers refuse the connection so Start can finish
// without leaking post-cancel stream loops.
func (s *Server) registerStreamConn(conn net.Conn) bool {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()

	if s.shuttingDown {
		return false
	}

	s.streamConns[conn] = struct{}{}
	s.streamWG.Add(1)
	return true
}

func (s *Server) unregisterStreamConn(conn net.Conn) {
	s.streamMu.Lock()
	delete(s.streamConns, conn)
	s.streamMu.Unlock()
	s.streamWG.Done()
}
