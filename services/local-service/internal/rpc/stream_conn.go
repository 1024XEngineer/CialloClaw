package rpc

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"sort"
	"sync"
)

// maxPendingStreamRequests caps per-connection in-flight work so a shared
// named-pipe stream applies backpressure before requests pile up behind task
// serialization locks.
const maxPendingStreamRequests = 32

type streamEnvelopeWriter struct {
	encoder *json.Encoder
	writeMu sync.Mutex
}

func (w *streamEnvelopeWriter) writeEnvelope(envelope any) error {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	return w.encoder.Encode(envelope)
}

type streamTaskCoordinator struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newStreamTaskCoordinator() *streamTaskCoordinator {
	return &streamTaskCoordinator{locks: map[string]*sync.Mutex{}}
}

// withTaskLocks serializes concurrent requests that target the same task while
// still letting unrelated requests share one desktop stream connection.
func (c *streamTaskCoordinator) withTaskLocks(taskIDs map[string]bool, fn func()) {
	if c == nil || len(taskIDs) == 0 {
		fn()
		return
	}

	orderedTaskIDs := make([]string, 0, len(taskIDs))
	for taskID := range taskIDs {
		orderedTaskIDs = append(orderedTaskIDs, taskID)
	}
	sort.Strings(orderedTaskIDs)

	locks := make([]*sync.Mutex, 0, len(orderedTaskIDs))
	c.mu.Lock()
	for _, taskID := range orderedTaskIDs {
		lock := c.locks[taskID]
		if lock == nil {
			lock = &sync.Mutex{}
			c.locks[taskID] = lock
		}
		locks = append(locks, lock)
	}
	c.mu.Unlock()

	for _, lock := range locks {
		lock.Lock()
	}
	defer func() {
		for index := len(locks) - 1; index >= 0; index-- {
			locks[index].Unlock()
		}
	}()

	fn()
}

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
	writer := &streamEnvelopeWriter{encoder: json.NewEncoder(conn)}
	taskCoordinator := newStreamTaskCoordinator()
	pendingRequests := make(chan struct{}, maxPendingStreamRequests)
	var taskStartRequestMu sync.Mutex

	for {
		var request requestEnvelope
		if err := decoder.Decode(&request); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}

			_ = writer.writeEnvelope(newErrorEnvelope(nil, &rpcError{
				Code:    errInvalidParams,
				Message: "INVALID_PARAMS",
				Detail:  "invalid json-rpc payload",
				TraceID: "trace_rpc_decode",
			}))
			return
		}

		pendingRequests <- struct{}{}
		s.streamWG.Add(1)
		go func(request requestEnvelope) {
			defer s.streamWG.Done()
			defer func() { <-pendingRequests }()
			s.handleStreamRequest(request, writer, taskCoordinator, &taskStartRequestMu)
		}(request)
	}
}

func (s *Server) handleStreamRequest(request requestEnvelope, writer *streamEnvelopeWriter, taskCoordinator *streamTaskCoordinator, taskStartMu *sync.Mutex) {
	tracker := newStreamRequestTracker(request)
	if tracker.shouldSubscribeTaskStart() {
		// Task-starting requests on one shared desktop stream must stay serialized.
		// Their runtime-notification correlation temporarily learns task ids from
		// task-start events, and parallel submits on the same connection can
		// otherwise cross-wire notifications before that mapping is established.
		taskStartMu.Lock()
		defer taskStartMu.Unlock()
	}

	taskCoordinator.withTaskLocks(tracker.taskIDsSnapshot(), func() {
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
				err := writer.writeEnvelope(newNotificationEnvelope(method, params))
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

		if err := writer.writeEnvelope(response); err != nil {
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
				if err := writer.writeEnvelope(newNotificationEnvelope(method, params)); err != nil {
					return
				}
			}
		}
	})
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
