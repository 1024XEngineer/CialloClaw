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

func (t *streamRequestTracker) taskIDsSnapshot() map[string]bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(t.taskIDs) == 0 {
		return nil
	}
	result := make(map[string]bool, len(t.taskIDs))
	for taskID, tracked := range t.taskIDs {
		if tracked {
			result[taskID] = true
		}
	}
	return result
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

// reserveStreamedRuntime records the buffered notification key before the
// transport write starts so replay checks never acquire tracker.mu after
// writeMu. The reservation is released when the write fails.
func (t *streamRequestTracker) reserveStreamedRuntime(method, taskID string, params map[string]any) string {
	key := notificationKey(method, taskID, params)

	t.mu.Lock()
	defer t.mu.Unlock()
	t.streamedRuntimeCounts[key]++
	return key
}

func (t *streamRequestTracker) releaseStreamedRuntimeReservation(key string) {
	if key == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.streamedRuntimeCounts[key] == 0 {
		return
	}
	t.streamedRuntimeCounts[key]--
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
