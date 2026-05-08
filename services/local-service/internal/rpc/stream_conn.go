package rpc

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

// maxPendingStreamRequests caps per-connection in-flight work so a shared
// named-pipe stream applies backpressure before requests pile up behind task
// serialization locks.
const maxPendingStreamRequests = 32

type streamConnState struct {
	closeOnce sync.Once
	closed    chan struct{}
}

func newStreamConnState() *streamConnState {
	return &streamConnState{
		closed: make(chan struct{}),
	}
}

func (s *streamConnState) close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		close(s.closed)
	})
}

func (s *streamConnState) isClosed() bool {
	if s == nil {
		return true
	}
	select {
	case <-s.closed:
		return true
	default:
		return false
	}
}

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

	reader := bufio.NewReader(conn)
	decoder := json.NewDecoder(reader)
	writer := &streamEnvelopeWriter{encoder: json.NewEncoder(conn)}
	connState := newStreamConnState()
	defer connState.close()
	taskCoordinator := newStreamTaskCoordinator()
	pendingRequests := make(chan struct{}, maxPendingStreamRequests)
	var taskStartRequestMu sync.Mutex

	for {
		// Acquire pending capacity before decoding the next request so a
		// disconnected client cannot leave behind a stale, already-decoded payload
		// that only starts after an in-flight worker frees a slot.
		pendingAcquireBlocked := false
		select {
		case pendingRequests <- struct{}{}:
		default:
			pendingAcquireBlocked = true
			pendingRequests <- struct{}{}
		}

		var request requestEnvelope
		if err := decoder.Decode(&request); err != nil {
			<-pendingRequests
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
		// If backpressure delayed decode until after the client had already
		// disconnected, drop this stale request instead of dispatching it after an
		// unrelated worker frees capacity.
		if pendingAcquireBlocked && streamReaderReachedEOF(reader, conn) {
			<-pendingRequests
			return
		}

		s.streamWG.Add(1)
		go func(request requestEnvelope) {
			defer s.streamWG.Done()
			defer func() { <-pendingRequests }()
			s.handleStreamRequest(request, writer, connState, taskCoordinator, &taskStartRequestMu)
		}(request)
	}
}

func streamReaderReachedEOF(reader *bufio.Reader, conn net.Conn) bool {
	if reader == nil || conn == nil {
		return false
	}
	if reader.Buffered() > 0 {
		return false
	}
	if err := conn.SetReadDeadline(time.Now()); err != nil {
		return false
	}
	defer conn.SetReadDeadline(time.Time{})

	_, err := reader.Peek(1)
	if errors.Is(err, io.EOF) {
		return true
	}
	var netErr net.Error
	return !errors.As(err, &netErr)
}

func (s *Server) handleStreamRequest(request requestEnvelope, writer *streamEnvelopeWriter, connState *streamConnState, taskCoordinator *streamTaskCoordinator, taskStartMu *sync.Mutex) {
	if connState.isClosed() {
		return
	}

	tracker := newStreamRequestTracker(request)
	initialTaskIDs := tracker.taskIDsSnapshot()
	if tracker.shouldSubscribeTaskStart() {
		// Task-starting requests on one shared desktop stream must stay serialized.
		// Their runtime-notification correlation temporarily learns task ids from
		// task-start events, and parallel submits on the same connection can
		// otherwise cross-wire notifications before that mapping is established.
		taskStartMu.Lock()
		defer taskStartMu.Unlock()
		if connState.isClosed() {
			return
		}
	}

	taskCoordinator.withTaskLocks(initialTaskIDs, func() {
		// Closing the stream must fence queued requests before dispatch so a dead
		// transport does not continue mutating task state from stale backlog.
		if connState.isClosed() {
			return
		}

		unsubscribeRuntime := func() {}
		if tracker.shouldSubscribeRuntime() {
			unsubscribeRuntime = s.orchestrator.SubscribeRuntimeNotifications(func(taskID string, method string, params map[string]any) {
				if connState.isClosed() {
					return
				}
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

		responseTaskIDs := taskIDsFromResponse(response)
		lateTaskIDs := responseTaskIDLocks(responseTaskIDs, initialTaskIDs)
		// Requests that only learn their task id from the response must claim the
		// real task lock before writing the response and destructively replaying the
		// buffered notification queue for that task.
		taskCoordinator.withTaskLocks(lateTaskIDs, func() {
			if connState.isClosed() {
				return
			}
			if err := writer.writeEnvelope(response); err != nil {
				return
			}

			for _, taskID := range responseTaskIDs {
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
	})
}

func responseTaskIDLocks(responseTaskIDs []string, initialTaskIDs map[string]bool) map[string]bool {
	if len(responseTaskIDs) == 0 {
		return nil
	}

	lateTaskIDs := make(map[string]bool, len(responseTaskIDs))
	for _, taskID := range responseTaskIDs {
		trimmed := strings.TrimSpace(taskID)
		if trimmed == "" || initialTaskIDs[trimmed] {
			continue
		}
		lateTaskIDs[trimmed] = true
	}
	if len(lateTaskIDs) == 0 {
		return nil
	}
	return lateTaskIDs
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
