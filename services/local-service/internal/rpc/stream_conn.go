package rpc

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
)

// handleStreamConn serves one long-lived JSON-RPC stream and replays task
// notifications on the same connection after the response envelope.
func (s *Server) handleStreamConn(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)
	var writeMu sync.Mutex

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

		streamedRuntimeCounts := map[string]int{}
		requestTaskIDs, requestSessionID, requestTraceID := requestRoutingHints(request)
		var requestTaskMu sync.RWMutex
		addRequestTaskID := func(taskID string) {
			trimmed := strings.TrimSpace(taskID)
			if trimmed == "" {
				return
			}
			requestTaskMu.Lock()
			if requestTaskIDs == nil {
				requestTaskIDs = map[string]bool{}
			}
			requestTaskIDs[trimmed] = true
			requestTaskMu.Unlock()
		}
		hasRequestTaskID := func(taskID string) bool {
			requestTaskMu.RLock()
			defer requestTaskMu.RUnlock()
			return requestTaskIDs != nil && requestTaskIDs[taskID]
		}
		matchesTaskStart := func(sessionID, traceID string) bool {
			switch {
			case requestTraceID != "":
				return requestTraceID == traceID
			case requestSessionID != "":
				return requestSessionID == sessionID
			default:
				return false
			}
		}

		unsubscribeRuntime := func() {}
		if requestTaskIDs != nil || shouldTrackStartedTask(request.Method) {
			unsubscribeRuntime = s.orchestrator.SubscribeRuntimeNotifications(func(taskID string, method string, params map[string]any) {
				if !isLiveRuntimeMethod(method) {
					return
				}
				notificationTaskID := runtimeNotificationTaskID(taskID, params)
				if notificationTaskID == "" || !hasRequestTaskID(notificationTaskID) {
					return
				}
				writeMu.Lock()
				defer writeMu.Unlock()
				if err := encoder.Encode(newNotificationEnvelope(method, params)); err == nil {
					streamedRuntimeCounts[notificationKey(method, notificationTaskID, params)]++
				}
			})
		}

		unsubscribeTaskStart := func() {}
		if shouldTrackStartedTask(request.Method) {
			unsubscribeTaskStart = s.orchestrator.SubscribeTaskStarts(func(taskID, sessionID, traceID string) {
				if !matchesTaskStart(sessionID, traceID) {
					return
				}
				addRequestTaskID(taskID)
			})
		}

		response := s.dispatch(request)
		unsubscribeTaskStart()
		unsubscribeRuntime()

		writeMu.Lock()
		err := encoder.Encode(response)
		writeMu.Unlock()
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
				key := notificationKey(method, taskID, params)
				if isLiveRuntimeMethod(method) && streamedRuntimeCounts[key] > 0 {
					streamedRuntimeCounts[key]--
					continue
				}
				writeMu.Lock()
				err := encoder.Encode(newNotificationEnvelope(method, params))
				writeMu.Unlock()
				if err != nil {
					return
				}
			}
		}
	}
}
