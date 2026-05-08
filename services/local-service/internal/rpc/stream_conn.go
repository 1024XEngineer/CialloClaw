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
	"sync/atomic"
	"time"
)

// maxPendingStreamRequests caps per-connection in-flight work so a shared
// named-pipe stream applies backpressure before requests pile up behind task
// serialization locks.
const maxPendingStreamRequests = 32

// blockedStreamDisconnectProbeWindow gives a briefly backpressured reader a
// chance to observe a peer disconnect before it dispatches a stale overflow
// request that was only decoded after capacity freed.
const blockedStreamDisconnectProbeWindow = 15 * time.Millisecond

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
	encoder      *json.Encoder
	onWriteError func()
	writeMu      sync.Mutex
}

func (w *streamEnvelopeWriter) writeEnvelope(envelope any) error {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	err := w.encoder.Encode(envelope)
	if err != nil && w.onWriteError != nil {
		w.onWriteError()
	}
	return err
}

type streamPendingState struct {
	blocked atomic.Bool
}

func (s *streamPendingState) setBlocked(blocked bool) {
	if s == nil {
		return
	}
	s.blocked.Store(blocked)
}

func (s *streamPendingState) isBlocked() bool {
	return s != nil && s.blocked.Load()
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
	connState := newStreamConnState()
	defer connState.close()
	writer := &streamEnvelopeWriter{
		encoder:      json.NewEncoder(conn),
		onWriteError: connState.close,
	}
	taskCoordinator := newStreamTaskCoordinator()
	pendingState := &streamPendingState{}
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
			pendingState.setBlocked(true)
			pendingRequests <- struct{}{}
			pendingState.setBlocked(false)
		}
		// Once a freed pending slot lets the read loop run again, probe for EOF
		// before the next decode so disconnected backlog can be fenced before any
		// same-task waiter is released.
		if pendingAcquireBlocked && streamReaderReachedEOF(reader, conn) {
			<-pendingRequests
			connState.close()
			return
		}

		var request requestEnvelope
		if err := decoder.Decode(&request); err != nil {
			<-pendingRequests
			connState.close()
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
			connState.close()
			return
		}

		s.streamWG.Add(1)
		go func(request requestEnvelope) {
			defer s.streamWG.Done()
			var pendingReleaseOnce sync.Once
			releasePending := func() {
				pendingReleaseOnce.Do(func() {
					<-pendingRequests
				})
			}
			defer releasePending()
			s.handleStreamRequest(request, writer, connState, pendingState, taskCoordinator, &taskStartRequestMu, releasePending)
		}(request)
	}
}

// streamReaderReachedEOF is only safe on the main stream read loop goroutine.
// It temporarily shortens read wait time to distinguish a disconnected peer
// from an idle shared stream before the next decode begins.
func streamReaderReachedEOF(reader *bufio.Reader, conn net.Conn) bool {
	if reader == nil || conn == nil {
		return false
	}
	if reader.Buffered() > 0 {
		return false
	}
	if err := conn.SetReadDeadline(time.Now().Add(blockedStreamDisconnectProbeWindow)); err != nil {
		return false
	}
	defer conn.SetReadDeadline(time.Time{})

	_, err := reader.Peek(1)
	if errors.Is(err, io.EOF) {
		return true
	}
	if err == nil {
		return false
	}
	var netErr net.Error
	return !errors.As(err, &netErr)
}

func (s *Server) handleStreamRequest(request requestEnvelope, writer *streamEnvelopeWriter, connState *streamConnState, pendingState *streamPendingState, taskCoordinator *streamTaskCoordinator, taskStartMu *sync.Mutex, releasePending func()) {
	if connState.isClosed() {
		return
	}

	tracker := newStreamRequestTracker(request)
	initialTaskIDs := tracker.taskIDsSnapshot()
	shouldProbeBlockedDisconnect := len(initialTaskIDs) > 0 || tracker.shouldSubscribeTaskStart()
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
		defer releasePending()

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
		ownedTaskIDs := ownedTaskIDsForReplay(request.Method, tracker.taskIDsSnapshot(), response)
		lateTaskIDs := responseTaskIDLocks(ownedTaskIDs, initialTaskIDs)
		// Requests that establish task ownership during dispatch must claim the real
		// task lock before writing the response and destructively replaying the
		// buffered notification queue for that task.
		taskCoordinator.withTaskLocks(lateTaskIDs, func() {
			if shouldProbeBlockedDisconnect && pendingState.isBlocked() {
				// Keep late-discovered task ownership fenced before freeing pending
				// capacity so a queued same-task follow-up cannot overtake the
				// response that established the task mapping on this shared stream.
				releasePending()
				select {
				case <-connState.closed:
					return
				case <-time.After(blockedStreamDisconnectProbeWindow):
				}
			}
			if connState.isClosed() {
				return
			}
			if err := writer.writeEnvelope(response); err != nil {
				return
			}

			for _, taskID := range ownedTaskIDs {
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
