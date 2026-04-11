package sidecarclient

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

type runtimePlaywrightClient struct {
	runtime *PlaywrightSidecarRuntime
}

func (c runtimePlaywrightClient) ReadPage(_ context.Context, _ string) (tools.BrowserPageReadResult, error) {
	if c.runtime == nil || !c.runtime.Ready() {
		return tools.BrowserPageReadResult{}, tools.ErrPlaywrightSidecarFailed
	}
	return tools.BrowserPageReadResult{
		URL:         "",
		Title:       "playwright sidecar ready",
		TextContent: "playwright sidecar transport skeleton is ready",
		MIMEType:    "text/plain",
		TextType:    "text/plain",
		Source:      "playwright_sidecar_ready_stub",
	}, nil
}

func (c runtimePlaywrightClient) SearchPage(_ context.Context, _, _ string, _ int) (tools.BrowserPageSearchResult, error) {
	if c.runtime == nil || !c.runtime.Ready() {
		return tools.BrowserPageSearchResult{}, tools.ErrPlaywrightSidecarFailed
	}
	return tools.BrowserPageSearchResult{
		URL:        "",
		Query:      "",
		MatchCount: 0,
		Matches:    []string{},
		Source:     "playwright_sidecar_ready_stub",
	}, nil
}

// PlaywrightSidecarRuntime 是当前阶段的最小运行时骨架。
//
// 它负责表达：
// - 当前 sidecar 名称
// - 当前 plugin 声明中是否存在该 sidecar
// - 当前最小 transport 规格
// - 当前 sidecar 是否已进入 ready 状态
type PlaywrightSidecarRuntime struct {
	mu     sync.Mutex
	spec   plugin.SidecarSpec
	os     platform.OSCapabilityAdapter
	ready  bool
	client runtimePlaywrightClient
}

// NewPlaywrightSidecarRuntime 创建并返回最小运行时骨架。
func NewPlaywrightSidecarRuntime(pluginService *plugin.Service, osCapability platform.OSCapabilityAdapter) (*PlaywrightSidecarRuntime, error) {
	spec, ok := pluginService.SidecarSpec("playwright_sidecar")
	if !ok {
		return nil, errors.New("playwright sidecar not declared")
	}
	runtime := &PlaywrightSidecarRuntime{
		spec: spec,
		os:   osCapability,
	}
	runtime.client = runtimePlaywrightClient{runtime: runtime}
	return runtime, nil
}

// Name 返回当前 sidecar 名称。
func (r *PlaywrightSidecarRuntime) Name() string {
	return r.spec.Name
}

// PipeName 返回当前最小传输骨架使用的命名管道名。
func (r *PlaywrightSidecarRuntime) PipeName() string {
	return sidecarPipeName(r.spec.Name)
}

// Ready 返回当前 sidecar 是否已进入 ready 状态。
func (r *PlaywrightSidecarRuntime) Ready() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ready
}

// Start 进入当前阶段的最小 ready 状态。
func (r *PlaywrightSidecarRuntime) Start() error {
	if r.os == nil {
		return errors.New("os capability adapter is required")
	}
	if err := r.os.EnsureNamedPipe(sidecarPipeName(r.spec.Name)); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ready = true
	return nil
}

// Stop 退出 ready 状态并关闭最小传输骨架。
func (r *PlaywrightSidecarRuntime) Stop() error {
	if r.os == nil {
		return nil
	}
	if err := r.os.CloseNamedPipe(sidecarPipeName(r.spec.Name)); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ready = false
	return nil
}

// Client 返回当前运行时关联的最小 client。
func (r *PlaywrightSidecarRuntime) Client() tools.PlaywrightSidecarClient {
	return r.client
}

func sidecarPipeName(name string) string {
	return fmt.Sprintf("cialloclaw-%s", name)
}
