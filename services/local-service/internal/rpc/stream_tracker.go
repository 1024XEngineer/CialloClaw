package rpc

import (
	"strings"
	"sync"
)

// streamRequestTracker keeps request-scoped notification routing state behind
// one lock so live runtime writes and buffered replay checks share invariants.
type streamRequestTracker struct {
	method                string
	taskIDs               map[string]bool
	sessionID             string
	traceID               string
	streamedRuntimeCounts map[string]int
	mu                    sync.RWMutex
}

func newStreamRequestTracker(request requestEnvelope) *streamRequestTracker {
	taskIDs, sessionID, traceID := requestRoutingHints(request)
	return &streamRequestTracker{
		method:                request.Method,
		taskIDs:               taskIDs,
		sessionID:             sessionID,
		traceID:               traceID,
		streamedRuntimeCounts: map[string]int{},
	}
}

func (t *streamRequestTracker) shouldSubscribeRuntime() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.taskIDs) > 0 || shouldTrackStartedTask(t.method)
}

func (t *streamRequestTracker) shouldSubscribeTaskStart() bool {
	return shouldTrackStartedTask(t.method)
}

func (t *streamRequestTracker) addTaskID(taskID string) {
	trimmed := strings.TrimSpace(taskID)
	if trimmed == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.taskIDs == nil {
		t.taskIDs = map[string]bool{}
	}
	t.taskIDs[trimmed] = true
}

func (t *streamRequestTracker) hasTaskID(taskID string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.taskIDs != nil && t.taskIDs[taskID]
}

func (t *streamRequestTracker) matchesTaskStart(sessionID, traceID string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	switch {
	case t.traceID != "":
		return t.traceID == traceID
	case t.sessionID != "":
		return t.sessionID == sessionID
	default:
		return false
	}
}

func (t *streamRequestTracker) recordStreamedRuntime(method, taskID string, params map[string]any) {
	key := notificationKey(method, taskID, params)

	t.mu.Lock()
	defer t.mu.Unlock()
	t.streamedRuntimeCounts[key]++
}

func (t *streamRequestTracker) shouldSkipBufferedRuntime(method, taskID string, params map[string]any) bool {
	if !isLiveRuntimeMethod(method) {
		return false
	}

	key := notificationKey(method, taskID, params)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.streamedRuntimeCounts[key] == 0 {
		return false
	}
	t.streamedRuntimeCounts[key]--
	return true
}
