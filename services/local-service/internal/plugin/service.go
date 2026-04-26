package plugin

import (
	"strings"
	"sync"
	"time"
)

type SidecarSpec struct {
	Name      string
	Transport string
}

// Manifest is the backend read model aligned with the formal PluginManifest
// contract so runtime visibility can point back to one versioned static asset.
type Manifest struct {
	PluginID     string
	Name         string
	Summary      string
	Version      string
	Entry        string
	Source       string
	Capabilities []string
	Permissions  []string
}

// RuntimeHealth represents the smallest stable health surface that later
// dashboard and query layers can consume without depending on concrete worker
// or sidecar implementations.
type RuntimeHealth string

const (
	RuntimeHealthUnknown     RuntimeHealth = "unknown"
	RuntimeHealthHealthy     RuntimeHealth = "healthy"
	RuntimeHealthDegraded    RuntimeHealth = "degraded"
	RuntimeHealthFailed      RuntimeHealth = "failed"
	RuntimeHealthStopped     RuntimeHealth = "stopped"
	RuntimeHealthUnavailable RuntimeHealth = "unavailable"
)

// RuntimeStatus separates lifecycle intent from health so callers can tell
// whether a plugin was never started, is active, or has been stopped.
type RuntimeStatus string

const (
	RuntimeStatusDeclared    RuntimeStatus = "declared"
	RuntimeStatusStarting    RuntimeStatus = "starting"
	RuntimeStatusRunning     RuntimeStatus = "running"
	RuntimeStatusStopped     RuntimeStatus = "stopped"
	RuntimeStatusUnavailable RuntimeStatus = "unavailable"
	RuntimeStatusFailed      RuntimeStatus = "failed"
)

// RuntimeKind distinguishes worker runtimes from sidecar runtimes.
type RuntimeKind string

const (
	RuntimeKindWorker  RuntimeKind = "worker"
	RuntimeKindSidecar RuntimeKind = "sidecar"
)

// RuntimeState is the smallest formal PluginRuntimeState read model exposed by
// the backend. It keeps runtime visibility separate from static declarations.
type RuntimeState struct {
	Name         string
	Kind         RuntimeKind
	Status       RuntimeStatus
	Transport    string
	Health       RuntimeHealth
	LastSeenAt   string
	LastError    string
	Manifest     *Manifest
	Capabilities []string
}

// MetricSnapshot is the smallest formal PluginMetricSnapshot read model used to
// report liveness and failure counts without introducing full telemetry yet.
type MetricSnapshot struct {
	Name          string
	Kind          RuntimeKind
	StartCount    int
	SuccessCount  int
	FailureCount  int
	LastStartedAt string
	LastFailedAt  string
	LastSeenAt    string
}

// RuntimeEvent represents one backend event payload that higher layers can fan
// out to dashboards or later persistence sinks.
type RuntimeEvent struct {
	Name      string
	Kind      RuntimeKind
	EventType string
	Payload   map[string]any
	CreatedAt string
}

const maxRuntimeEvents = 50

// Service keeps static declarations plus the current runtime state cache.
type Service struct {
	mu        sync.Mutex
	order     []string
	manifests map[string]Manifest
	runtimes  map[string]RuntimeState
	metrics   map[string]MetricSnapshot
	events    []RuntimeEvent
}

// NewService creates the plugin runtime registry with declared workers and sidecars.
func NewService() *Service {
	service := &Service{
		order:     make([]string, 0),
		manifests: map[string]Manifest{},
		runtimes:  map[string]RuntimeState{},
		metrics:   map[string]MetricSnapshot{},
		events:    make([]RuntimeEvent, 0),
	}
	entries := BuiltinCatalogEntries()
	for _, entry := range entries {
		manifest := manifestFromCatalogEntry(entry)
		for _, ref := range entry.RuntimeRefs {
			if ref.Kind != RuntimeKindWorker {
				continue
			}
			service.declareRuntime(RuntimeState{
				Name:         ref.Name,
				Kind:         ref.Kind,
				Status:       RuntimeStatusDeclared,
				Transport:    ref.Transport,
				Health:       RuntimeHealthUnknown,
				Manifest:     &manifest,
				Capabilities: append([]string(nil), entry.Capabilities...),
			})
		}
	}
	for _, entry := range entries {
		manifest := manifestFromCatalogEntry(entry)
		for _, ref := range entry.RuntimeRefs {
			if ref.Kind != RuntimeKindSidecar {
				continue
			}
			service.declareRuntime(RuntimeState{
				Name:         ref.Name,
				Kind:         ref.Kind,
				Status:       RuntimeStatusDeclared,
				Transport:    ref.Transport,
				Health:       RuntimeHealthUnknown,
				Manifest:     &manifest,
				Capabilities: append([]string(nil), entry.Capabilities...),
			})
		}
	}
	return service
}

func (s *Service) declareRuntime(state RuntimeState) {
	key := runtimeKey(state.Kind, state.Name)
	s.order = append(s.order, key)
	if state.Manifest != nil && strings.TrimSpace(state.Manifest.PluginID) != "" {
		s.manifests[strings.TrimSpace(state.Manifest.PluginID)] = cloneManifest(state.Manifest)
	}
	s.runtimes[key] = state
	s.metrics[key] = MetricSnapshot{Name: state.Name, Kind: state.Kind}
}

// Manifests returns the current static plugin manifest catalog in stable order.
func (s *Service) Manifests() []Manifest {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]Manifest, 0, len(s.manifests))
	seen := map[string]struct{}{}
	for _, key := range s.order {
		runtime, ok := s.runtimes[key]
		if !ok || runtime.Manifest == nil {
			continue
		}
		pluginID := strings.TrimSpace(runtime.Manifest.PluginID)
		if pluginID == "" {
			continue
		}
		if _, ok := seen[pluginID]; ok {
			continue
		}
		seen[pluginID] = struct{}{}
		result = append(result, cloneManifest(runtime.Manifest))
	}
	return result
}

func (s *Service) Workers() []string {
	return s.runtimeNamesByKind(RuntimeKindWorker)
}

func (s *Service) Sidecars() []string {
	return s.runtimeNamesByKind(RuntimeKindSidecar)
}

func (s *Service) runtimeNamesByKind(kind RuntimeKind) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]string, 0)
	for _, key := range s.order {
		runtime, ok := s.runtimes[key]
		if !ok {
			continue
		}
		if runtime.Kind == kind {
			result = append(result, runtime.Name)
		}
	}
	return result
}

func (s *Service) HasSidecar(name string) bool {
	needle := strings.TrimSpace(name)
	if needle == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.runtimes[runtimeKey(RuntimeKindSidecar, needle)]
	return ok
}

func (s *Service) PrimarySidecar() string {
	if sidecars := s.Sidecars(); len(sidecars) > 0 {
		return sidecars[0]
	}
	return ""
}

func (s *Service) SidecarSpec(name string) (SidecarSpec, bool) {
	if !s.HasSidecar(name) {
		return SidecarSpec{}, false
	}
	state, ok := s.RuntimeState(RuntimeKindSidecar, name)
	if !ok {
		return SidecarSpec{}, false
	}
	return SidecarSpec{Name: state.Name, Transport: state.Transport}, true
}

// RuntimeState returns one runtime entry by kind/name.
func (s *Service) RuntimeState(kind RuntimeKind, name string) (RuntimeState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.runtimes[runtimeKey(kind, name)]
	return cloneRuntimeState(state), ok
}

// RuntimeStates returns all currently known runtime entries.
func (s *Service) RuntimeStates() []RuntimeState {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]RuntimeState, 0, len(s.runtimes))
	for _, key := range s.order {
		runtime, ok := s.runtimes[key]
		if !ok {
			continue
		}
		result = append(result, cloneRuntimeState(runtime))
	}
	return result
}

// MetricSnapshots returns all current metric snapshots.
func (s *Service) MetricSnapshots() []MetricSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]MetricSnapshot, 0, len(s.metrics))
	for _, key := range s.order {
		metric, ok := s.metrics[key]
		if !ok {
			continue
		}
		result = append(result, metric)
	}
	return result
}

// RuntimeEvents returns the buffered runtime event snapshots.
func (s *Service) RuntimeEvents() []RuntimeEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]RuntimeEvent, 0, len(s.events))
	for _, event := range s.events {
		result = append(result, RuntimeEvent{Name: event.Name, Kind: event.Kind, EventType: event.EventType, Payload: cloneMap(event.Payload), CreatedAt: event.CreatedAt})
	}
	return result
}

// MarkRuntimeStarting records that one runtime is attempting to start.
func (s *Service) MarkRuntimeStarting(kind RuntimeKind, name string) {
	s.updateRuntime(kind, name, RuntimeStatusStarting, RuntimeHealthUnknown, "")
	s.bumpMetric(kind, name, func(metric *MetricSnapshot) {
		metric.StartCount++
		metric.LastStartedAt = time.Now().UTC().Format(time.RFC3339)
	})
	s.appendEvent(kind, name, "plugin.runtime.starting", map[string]any{"status": RuntimeStatusStarting})
}

// MarkRuntimeHealthy records a healthy runtime heartbeat.
func (s *Service) MarkRuntimeHealthy(kind RuntimeKind, name string) {
	s.updateRuntime(kind, name, RuntimeStatusRunning, RuntimeHealthHealthy, "")
	s.bumpMetric(kind, name, func(metric *MetricSnapshot) {
		metric.SuccessCount++
		metric.LastSeenAt = time.Now().UTC().Format(time.RFC3339)
	})
	s.appendEvent(kind, name, "plugin.runtime.healthy", map[string]any{"health": RuntimeHealthHealthy})
}

// MarkRuntimeFailed records a runtime failure.
func (s *Service) MarkRuntimeFailed(kind RuntimeKind, name string, err error) {
	errorText := "runtime failed"
	if err != nil {
		errorText = strings.TrimSpace(err.Error())
	}
	s.updateRuntime(kind, name, RuntimeStatusFailed, RuntimeHealthFailed, errorText)
	s.bumpMetric(kind, name, func(metric *MetricSnapshot) {
		metric.FailureCount++
		metric.LastFailedAt = time.Now().UTC().Format(time.RFC3339)
		metric.LastSeenAt = time.Now().UTC().Format(time.RFC3339)
	})
	s.appendEvent(kind, name, "plugin.runtime.failed", map[string]any{"error": errorText})
}

// MarkRuntimeUnavailable records that a declared runtime cannot be started.
func (s *Service) MarkRuntimeUnavailable(kind RuntimeKind, name string, reason string) {
	s.updateRuntime(kind, name, RuntimeStatusUnavailable, RuntimeHealthUnavailable, strings.TrimSpace(reason))
	s.appendEvent(kind, name, "plugin.runtime.unavailable", map[string]any{"error": strings.TrimSpace(reason)})
}

// MarkRuntimeStopped records a graceful stop signal.
func (s *Service) MarkRuntimeStopped(kind RuntimeKind, name string) {
	s.updateRuntime(kind, name, RuntimeStatusStopped, RuntimeHealthStopped, "")
	s.appendEvent(kind, name, "plugin.runtime.stopped", map[string]any{"status": RuntimeStatusStopped})
}

func (s *Service) updateRuntime(kind RuntimeKind, name string, status RuntimeStatus, health RuntimeHealth, lastError string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := runtimeKey(kind, name)
	state := s.runtimes[key]
	state.Status = status
	state.Health = health
	state.LastSeenAt = time.Now().UTC().Format(time.RFC3339)
	state.LastError = strings.TrimSpace(lastError)
	s.runtimes[key] = state
	metric := s.metrics[key]
	metric.Name = state.Name
	metric.Kind = state.Kind
	metric.LastSeenAt = state.LastSeenAt
	s.metrics[key] = metric
}

func (s *Service) bumpMetric(kind RuntimeKind, name string, fn func(metric *MetricSnapshot)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := runtimeKey(kind, name)
	metric := s.metrics[key]
	metric.Name = name
	metric.Kind = kind
	fn(&metric)
	s.metrics[key] = metric
}

func (s *Service) appendEvent(kind RuntimeKind, name string, eventType string, payload map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, RuntimeEvent{
		Name:      name,
		Kind:      kind,
		EventType: eventType,
		Payload:   cloneMap(payload),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if len(s.events) > maxRuntimeEvents {
		s.events = append([]RuntimeEvent(nil), s.events[len(s.events)-maxRuntimeEvents:]...)
	}
}

func runtimeKey(kind RuntimeKind, name string) string {
	return string(kind) + "::" + strings.TrimSpace(name)
}

func cloneRuntimeState(state RuntimeState) RuntimeState {
	return RuntimeState{
		Name:         state.Name,
		Kind:         state.Kind,
		Status:       state.Status,
		Transport:    state.Transport,
		Health:       state.Health,
		LastSeenAt:   state.LastSeenAt,
		LastError:    state.LastError,
		Manifest:     cloneManifestPointer(state.Manifest),
		Capabilities: append([]string(nil), state.Capabilities...),
	}
}

func cloneManifestPointer(manifest *Manifest) *Manifest {
	if manifest == nil {
		return nil
	}
	cloned := cloneManifest(manifest)
	return &cloned
}

func cloneManifest(manifest *Manifest) Manifest {
	if manifest == nil {
		return Manifest{}
	}
	return Manifest{
		PluginID:     manifest.PluginID,
		Name:         manifest.Name,
		Summary:      manifest.Summary,
		Version:      manifest.Version,
		Entry:        manifest.Entry,
		Source:       manifest.Source,
		Capabilities: append([]string(nil), manifest.Capabilities...),
		Permissions:  append([]string(nil), manifest.Permissions...),
	}
}

func cloneMap(payload map[string]any) map[string]any {
	if len(payload) == 0 {
		return nil
	}
	result := make(map[string]any, len(payload))
	for key, value := range payload {
		result[key] = value
	}
	return result
}

func stringValue(payload map[string]any, key string) string {
	if len(payload) == 0 {
		return ""
	}
	value, _ := payload[key].(string)
	return strings.TrimSpace(value)
}
