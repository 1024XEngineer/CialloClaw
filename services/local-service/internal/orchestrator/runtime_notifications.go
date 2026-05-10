package orchestrator

// SubscribeRuntimeNotifications registers a temporary tap for execution-time
// runtime notifications so transports can mirror in-flight loop events without
// waiting for the enclosing RPC response to finish.
func (s *Service) SubscribeRuntimeNotifications(listener func(taskID, method string, params map[string]any)) func() {
	if s == nil || listener == nil {
		return func() {}
	}

	s.runtimeMu.Lock()
	s.runtimeNextID++
	listenerID := s.runtimeNextID
	s.runtimeTaps[listenerID] = listener
	s.runtimeMu.Unlock()

	return func() {
		s.runtimeMu.Lock()
		delete(s.runtimeTaps, listenerID)
		s.runtimeMu.Unlock()
	}
}

// SubscribeTaskStarts registers a temporary tap that reports newly created
// tasks before execution continues, allowing transports to associate follow-on
// runtime notifications with requests that did not yet know their task_id.
func (s *Service) SubscribeTaskStarts(listener func(taskID, sessionID, traceID string)) func() {
	if s == nil || listener == nil {
		return func() {}
	}

	s.runtimeMu.Lock()
	s.runtimeNextID++
	listenerID := s.runtimeNextID
	s.taskStartTaps[listenerID] = listener
	s.runtimeMu.Unlock()

	return func() {
		s.runtimeMu.Lock()
		delete(s.taskStartTaps, listenerID)
		s.runtimeMu.Unlock()
	}
}

func (s *Service) publishRuntimeNotification(taskID, method string, params map[string]any) {
	if s == nil {
		return
	}

	s.runtimeMu.RLock()
	if len(s.runtimeTaps) == 0 {
		s.runtimeMu.RUnlock()
		return
	}
	listeners := make([]func(taskID, method string, params map[string]any), 0, len(s.runtimeTaps))
	for _, listener := range s.runtimeTaps {
		listeners = append(listeners, listener)
	}
	s.runtimeMu.RUnlock()

	for _, listener := range listeners {
		listener(taskID, method, cloneMap(params))
	}
}

func (s *Service) publishTaskStart(taskID, sessionID, traceID string) {
	if s == nil {
		return
	}

	s.runtimeMu.RLock()
	if len(s.taskStartTaps) == 0 {
		s.runtimeMu.RUnlock()
		return
	}
	listeners := make([]func(taskID, sessionID, traceID string), 0, len(s.taskStartTaps))
	for _, listener := range s.taskStartTaps {
		listeners = append(listeners, listener)
	}
	s.runtimeMu.RUnlock()

	for _, listener := range listeners {
		listener(taskID, sessionID, traceID)
	}
}

// PendingNotifications returns the buffered notification list for a task
// without consuming it. Debug transports use this read-only path when they need
// to inspect pending events but must not disturb the ordered replay pipeline.
func (s *Service) PendingNotifications(taskID string) ([]map[string]any, error) {
	notifications, ok := s.runEngine.PendingNotifications(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}

	items := make([]map[string]any, 0, len(notifications))
	for _, notification := range notifications {
		items = append(items, map[string]any{
			"method":     notification.Method,
			"params":     cloneMap(notification.Params),
			"created_at": notification.CreatedAt.Format(dateTimeLayout),
		})
	}

	return items, nil
}

// DrainNotifications returns and clears the buffered notification list for a
// task. The orchestrator exposes this explicit destructive read so transports
// can replay notifications exactly once instead of coupling queue semantics to
// ordinary task detail or list reads.
func (s *Service) DrainNotifications(taskID string) ([]map[string]any, error) {
	notifications, ok := s.runEngine.DrainNotifications(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}

	items := make([]map[string]any, 0, len(notifications))
	for _, notification := range notifications {
		items = append(items, map[string]any{
			"method":     notification.Method,
			"params":     cloneMap(notification.Params),
			"created_at": notification.CreatedAt.Format(dateTimeLayout),
		})
	}

	return items, nil
}
