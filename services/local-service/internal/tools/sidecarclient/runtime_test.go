package sidecarclient

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

type stubWorkerInvoker struct {
	response sidecarResponse
	err      error
	delay    time.Duration
	requests []sidecarRequest
}

func (s *stubWorkerInvoker) Invoke(ctx context.Context, request sidecarRequest) (sidecarResponse, error) {
	s.requests = append(s.requests, request)
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return sidecarResponse{}, ctx.Err()
		}
	}
	if s.err != nil {
		return sidecarResponse{}, s.err
	}
	return s.response, nil
}

func writeTempWorkerScript(t *testing.T, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "worker.js")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write temp worker script: %v", err)
	}
	return path
}

func TestPlaywrightSidecarRuntimeLifecycle(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	runtime.invoker = &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{"status": "ok"}}}
	if runtime.Name() != "playwright_sidecar" {
		t.Fatalf("unexpected runtime name: %q", runtime.Name())
	}
	if runtime.Ready() {
		t.Fatal("expected runtime to start as not ready")
	}
	if err := runtime.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if !runtime.Ready() {
		t.Fatal("expected runtime to be ready after start")
	}
	if !osCapability.HasNamedPipe(runtime.PipeName()) {
		t.Fatal("expected named pipe to be registered after start")
	}
	if err := runtime.Stop(); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	if runtime.Ready() {
		t.Fatal("expected runtime to be not ready after stop")
	}
}

func TestPlaywrightSidecarRuntimeClientReturnsCapabilityErrorWhenNotReady(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	_, err = runtime.Client().ReadPage(t.Context(), "https://example.com")
	if err != tools.ErrPlaywrightSidecarFailed {
		t.Fatalf("expected ErrPlaywrightSidecarFailed, got %v", err)
	}
}

func TestPlaywrightSidecarRuntimeClientExecutesRealReadAndSearch(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	invoker := &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{
		"url":          "https://example.com",
		"title":        "Example Domain",
		"text_content": "example text",
		"mime_type":    "text/html",
		"text_type":    "text/html",
		"source":       "playwright_worker_http",
	}}}
	runtime.invoker = invoker
	if err := runtime.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	readResult, err := runtime.Client().ReadPage(t.Context(), "https://example.com")
	if err != nil {
		t.Fatalf("ReadPage returned error: %v", err)
	}
	if readResult.Title != "Example Domain" || readResult.Source != "playwright_worker_http" {
		t.Fatalf("unexpected read result: %+v", readResult)
	}
	invoker.response = sidecarResponse{OK: true, Result: map[string]any{
		"url":         "https://example.com",
		"query":       "example",
		"match_count": 1,
		"matches":     []any{"example text"},
		"source":      "playwright_worker_http",
	}}
	searchResult, err := runtime.Client().SearchPage(t.Context(), "https://example.com", "example", 3)
	if err != nil {
		t.Fatalf("SearchPage returned error: %v", err)
	}
	if searchResult.MatchCount != 1 || len(searchResult.Matches) != 1 {
		t.Fatalf("unexpected search result: %+v", searchResult)
	}
	if len(invoker.requests) < 3 || invoker.requests[1].Action != "page_read" || invoker.requests[2].Action != "page_search" {
		t.Fatalf("unexpected request sequence: %+v", invoker.requests)
	}
}

func TestPlaywrightSidecarRuntimeStartFailsHealthCheck(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	runtime.invoker = &stubWorkerInvoker{err: errors.New("health failed")}
	if err := runtime.Start(); !errors.Is(err, tools.ErrPlaywrightSidecarFailed) {
		t.Fatalf("expected sidecar failure from health check, got %v", err)
	}
	if runtime.Ready() {
		t.Fatal("expected runtime not ready after failed health check")
	}
}

func TestPlaywrightSidecarRuntimeRequestFailureKeepsReadyState(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	runtime.invoker = &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{"status": "ok"}}}
	if err := runtime.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	runtime.invoker = &stubWorkerInvoker{err: sidecarRequestError{code: "http_404", message: "page not found"}}
	_, err = runtime.Client().ReadPage(t.Context(), "https://example.com")
	if !errors.Is(err, tools.ErrPlaywrightSidecarFailed) {
		t.Fatalf("expected wrapped sidecar failure, got %v", err)
	}
	if !runtime.Ready() {
		t.Fatal("expected runtime to stay ready after request failure")
	}
	if !osCapability.HasNamedPipe(runtime.PipeName()) {
		t.Fatal("expected named pipe to stay registered after request failure")
	}
}

func TestPlaywrightSidecarRuntimeTransportFailureClearsReadyState(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	runtime.invoker = &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{"status": "ok"}}}
	if err := runtime.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	runtime.invoker = &stubWorkerInvoker{err: sidecarTransportError{err: errors.New("worker crashed")}}
	_, err = runtime.Client().ReadPage(t.Context(), "https://example.com")
	if !errors.Is(err, tools.ErrPlaywrightSidecarFailed) {
		t.Fatalf("expected wrapped sidecar failure, got %v", err)
	}
	if runtime.Ready() {
		t.Fatal("expected runtime to leave ready state after transport failure")
	}
	if osCapability.HasNamedPipe(runtime.PipeName()) {
		t.Fatal("expected named pipe to be closed after transport failure")
	}
}

func TestPlaywrightSidecarRuntimeInvokeTimeoutKeepsReadyState(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	runtime.invoker = &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{"status": "ok"}}}
	if err := runtime.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	runtime.invoker = &stubWorkerInvoker{delay: 20 * time.Millisecond}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Millisecond)
	defer cancel()
	_, err = runtime.Client().ReadPage(ctx, "https://example.com")
	if !errors.Is(err, tools.ErrPlaywrightSidecarFailed) {
		t.Fatalf("expected sidecar failure on timeout, got %v", err)
	}
	if !runtime.Ready() {
		t.Fatal("expected runtime to stay ready after request timeout")
	}
}

func TestResolveRelativePathFromRootsFindsWorkerEntry(t *testing.T) {
	root := t.TempDir()
	entryPath := filepath.Join(root, "workers", "playwright-worker", "src", "index.js")
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o755); err != nil {
		t.Fatalf("mkdir worker path: %v", err)
	}
	if err := os.WriteFile(entryPath, []byte("console.log('ok')\n"), 0o644); err != nil {
		t.Fatalf("write worker entry: %v", err)
	}
	resolved, err := resolveRelativePathFromRoots(playwrightWorkerRelativePath, []string{filepath.Join(root, "services", "local-service")})
	if err != nil {
		t.Fatalf("resolveRelativePathFromRoots returned error: %v", err)
	}
	if resolved != entryPath {
		t.Fatalf("expected resolved entry %q, got %q", entryPath, resolved)
	}
	if _, err := resolveRelativePathFromRoots(playwrightWorkerRelativePath, []string{filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("expected missing worker entry lookup to fail")
	}
}

func TestCommandWorkerInvokerInvokeReturnsStructuredRequestError(t *testing.T) {
	entryPath := writeTempWorkerScript(t, `process.stdin.resume(); process.stdin.on("end", () => { process.stdout.write(JSON.stringify({ ok: false, error: { code: "http_404", message: "page not found" } })); process.exitCode = 1; });`)
	invoker := newCommandWorkerInvoker(entryPath)
	response, err := invoker.Invoke(context.Background(), sidecarRequest{Action: "page_read", URL: "https://example.com"})
	if response.OK {
		t.Fatalf("expected request error response, got %+v", response)
	}
	var requestErr sidecarRequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("expected request error, got %v", err)
	}
	if requestErr.code != "http_404" {
		t.Fatalf("expected request error code http_404, got %+v", requestErr)
	}
	if shouldMarkRuntimeFailure(err) {
		t.Fatal("expected request error not to mark runtime failure")
	}
}

func TestCommandWorkerInvokerInvokeReturnsTransportErrorForInvalidOutput(t *testing.T) {
	entryPath := writeTempWorkerScript(t, `process.stderr.write("crashed\n"); process.stdout.write("not-json"); process.exitCode = 1;`)
	invoker := newCommandWorkerInvoker(entryPath)
	_, err := invoker.Invoke(context.Background(), sidecarRequest{Action: "health"})
	var transportErr sidecarTransportError
	if !errors.As(err, &transportErr) {
		t.Fatalf("expected transport error, got %v", err)
	}
	if !strings.Contains(err.Error(), "crashed") {
		t.Fatalf("expected stderr in transport error, got %v", err)
	}
	if !shouldMarkRuntimeFailure(err) {
		t.Fatal("expected transport error to mark runtime failure")
	}
}

func TestCommandWorkerInvokerInvokeTimeoutReturnsRequestError(t *testing.T) {
	entryPath := writeTempWorkerScript(t, `setTimeout(() => { process.stdout.write(JSON.stringify({ ok: true, result: { status: "late" } })); }, 200);`)
	invoker := newCommandWorkerInvoker(entryPath)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := invoker.Invoke(ctx, sidecarRequest{Action: "health"})
	var requestErr sidecarRequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("expected timeout request error, got %v", err)
	}
	if requestErr.code != "timeout" {
		t.Fatalf("expected timeout request error code, got %+v", requestErr)
	}
}
