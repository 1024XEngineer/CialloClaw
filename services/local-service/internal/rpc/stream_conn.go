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
				writeMu.Lock()
				err := encoder.Encode(newNotificationEnvelope(method, params))
				writeMu.Unlock()
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
				if tracker.shouldSkipBufferedRuntime(method, taskID, params) {
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
