// Package bootstrap assembles local-service dependencies and startup wiring.
package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/delivery"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/execution"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/memory"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/orchestrator"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/risk"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/rpc"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskinspector"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/builtin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/sidecarclient"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/traceeval"
)

// App keeps the assembled local-service runtime dependencies.
type App struct {
	server       *rpc.Server
	storage      *storage.Service
	toolRegistry *tools.Registry
	toolExecutor *tools.ToolExecutor
	playwright   *sidecarclient.PlaywrightSidecarRuntime
	ocr          *sidecarclient.OCRWorkerRuntime
	media        *sidecarclient.MediaWorkerRuntime
}

type runtimeStarter interface {
	Start() error
}

var (
	newLocalPathPolicyForBootstrap        = platform.NewLocalPathPolicy
	registerBuiltinToolsForBootstrap      = builtin.RegisterBuiltinTools
	registerPlaywrightToolsForBootstrap   = sidecarclient.RegisterPlaywrightTools
	registerOCRToolsForBootstrap          = sidecarclient.RegisterOCRTools
	registerMediaToolsForBootstrap        = sidecarclient.RegisterMediaTools
	newModelServiceFromConfigForBootstrap = model.NewServiceFromConfig
)

// New assembles a fully wired local-service application.
func New(cfg config.Config) (*App, error) {
	if strings.ContainsRune(cfg.WorkspaceRoot, '\x00') {
		return nil, fmt.Errorf("workspace root contains invalid null byte")
	}

	pathPolicy, err := newLocalPathPolicyForBootstrap(cfg.WorkspaceRoot)
	if err != nil {
		return nil, err
	}

	storageService := storage.NewService(platform.NewLocalStorageAdapter(cfg.DatabasePath))
	auditService := audit.NewService(storageService.AuditWriter())
	checkpointService := checkpoint.NewService(storageService.RecoveryPointWriter())
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	executionBackend := platform.NewControlledExecutionBackend(cfg.WorkspaceRoot)
	osCapability := platform.NewLocalOSCapabilityAdapter()
	pluginService := plugin.NewService()
	if err := storageService.EnsureBuiltinExecutionAssets(context.Background()); err != nil {
		_ = storageService.Close()
		return nil, err
	}
	if err := persistPluginManifests(context.Background(), storageService, pluginService); err != nil {
		_ = storageService.Close()
		return nil, err
	}
	playwrightRuntime, err := sidecarclient.NewPlaywrightSidecarRuntime(pluginService, osCapability)
	playwrightRuntime = chooseRuntimeOnStart(playwrightRuntime, err, func() *sidecarclient.PlaywrightSidecarRuntime {
		return sidecarclient.NewUnavailablePlaywrightSidecarRuntime(pluginService, osCapability)
	})
	playwrightClient := playwrightRuntime.Client()
	ocrRuntime, err := sidecarclient.NewOCRWorkerRuntime(pluginService, osCapability)
	ocrRuntime = chooseRuntimeOnStart(ocrRuntime, err, func() *sidecarclient.OCRWorkerRuntime {
		return sidecarclient.NewUnavailableOCRWorkerRuntime(pluginService, osCapability)
	})
	ocrClient := ocrRuntime.Client()
	mediaRuntime, err := sidecarclient.NewMediaWorkerRuntime(pluginService, osCapability)
	mediaRuntime = chooseRuntimeOnStart(mediaRuntime, err, func() *sidecarclient.MediaWorkerRuntime {
		return sidecarclient.NewUnavailableMediaWorkerRuntime(pluginService, osCapability)
	})
	mediaClient := mediaRuntime.Client()
	screenClient := sidecarclient.NewLocalScreenCaptureClient(fileSystem)
	toolRegistry := tools.NewRegistry()
	if err := registerBuiltinToolsForBootstrap(toolRegistry); err != nil {
		_ = storageService.Close()
		return nil, err
	}
	if err := registerPlaywrightToolsForBootstrap(toolRegistry); err != nil {
		_ = storageService.Close()
		return nil, err
	}
	if err := registerOCRToolsForBootstrap(toolRegistry); err != nil {
		_ = storageService.Close()
		return nil, err
	}
	if err := registerMediaToolsForBootstrap(toolRegistry); err != nil {
		_ = storageService.Close()
		return nil, err
	}
	toolExecutor := tools.NewToolExecutor(
		toolRegistry,
		tools.WithToolCallRecorder(tools.NewToolCallRecorder(storageService.ToolCallSink())),
	)

	modelService, err := newModelServiceFromConfigForBootstrap(model.ServiceConfig{
		ModelConfig:  cfg.Model,
		SecretSource: model.NewStaticSecretSource(storageService),
	})
	if err != nil {
		if errors.Is(err, model.ErrSecretSourceFailed) && (errors.Is(err, model.ErrSecretNotFound) || errors.Is(err, storage.ErrSecretNotFound)) {
			modelService = model.NewService(cfg.Model)
		} else {
			_ = storageService.Close()
			return nil, err
		}
	}

	deliveryService := delivery.NewService()
	traceEvalService := traceeval.NewService(storageService.TraceStore(), storageService.EvalStore())
	executionService := execution.NewService(fileSystem, executionBackend, playwrightClient, ocrClient, mediaClient, screenClient, modelService, auditService, checkpointService, deliveryService, toolRegistry, toolExecutor, pluginService).
		WithArtifactStore(storageService.ArtifactStore()).
		WithLoopRuntimeStore(storageService.LoopRuntimeStore()).
		WithExtensionAssetCatalog(storageService)
	inspectorService := taskinspector.NewService(fileSystem)
	runEngine, err := runengine.NewEngineWithStore(storageService.TaskRunStore())
	if err != nil {
		_ = storageService.Close()
		return nil, err
	}
	if err := runEngine.WithTodoStore(storageService.TodoStore()); err != nil {
		_ = storageService.Close()
		return nil, err
	}
	if err := runEngine.WithSettingsStore(storageService.SettingsStore()); err != nil {
		_ = storageService.Close()
		return nil, err
	}
	if err := runEngine.WithSessionStore(storageService.SessionStore()); err != nil {
		_ = storageService.Close()
		return nil, err
	}

	orchestratorService := orchestrator.NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runEngine,
		deliveryService,
		memory.NewServiceFromStorage(storageService.MemoryStore(), storageService.Capabilities().MemoryRetrievalBackend),
		risk.NewService(),
		modelService,
		toolRegistry,
		pluginService,
	).WithAudit(auditService).WithExecutor(executionService).WithStorage(storageService).WithTaskInspector(inspectorService).WithTraceEval(traceEvalService)

	return &App{
		server:       rpc.NewServer(cfg.RPC, orchestratorService),
		storage:      storageService,
		toolRegistry: toolRegistry,
		toolExecutor: toolExecutor,
		playwright:   playwrightRuntime,
		ocr:          ocrRuntime,
		media:        mediaRuntime,
	}, nil
}

func persistPluginManifests(ctx context.Context, storageService *storage.Service, pluginService *plugin.Service) error {
	if storageService == nil || pluginService == nil || storageService.PluginManifestStore() == nil {
		return nil
	}
	timestamp := time.Now().UTC().Format(time.RFC3339)
	runtimeNamesByPluginID := map[string][]string{}
	for _, runtime := range pluginService.RuntimeStates() {
		if runtime.Manifest == nil || runtime.Manifest.PluginID == "" {
			continue
		}
		runtimeNamesByPluginID[runtime.Manifest.PluginID] = append(runtimeNamesByPluginID[runtime.Manifest.PluginID], runtime.Name)
	}
	for _, manifest := range pluginService.Manifests() {
		capabilitiesJSON, err := json.Marshal(manifest.Capabilities)
		if err != nil {
			return fmt.Errorf("marshal plugin manifest capabilities for %s: %w", manifest.PluginID, err)
		}
		permissionsJSON, err := json.Marshal(manifest.Permissions)
		if err != nil {
			return fmt.Errorf("marshal plugin manifest permissions for %s: %w", manifest.PluginID, err)
		}
		runtimeNamesJSON, err := json.Marshal(runtimeNamesByPluginID[manifest.PluginID])
		if err != nil {
			return fmt.Errorf("marshal plugin manifest runtime names for %s: %w", manifest.PluginID, err)
		}
		record := storage.PluginManifestRecord{
			PluginID:         manifest.PluginID,
			Name:             manifest.Name,
			Version:          manifest.Version,
			Entry:            manifest.Entry,
			Source:           manifest.Source,
			Summary:          firstNonEmptyBootstrap(strings.TrimSpace(manifest.Summary), fmt.Sprintf("Built-in plugin manifest for %s.", manifest.Name)),
			CapabilitiesJSON: string(capabilitiesJSON),
			PermissionsJSON:  string(permissionsJSON),
			RuntimeNamesJSON: string(runtimeNamesJSON),
			CreatedAt:        timestamp,
			UpdatedAt:        timestamp,
		}
		if err := storageService.PluginManifestStore().WritePluginManifest(ctx, record); err != nil {
			return fmt.Errorf("write plugin manifest %s: %w", manifest.PluginID, err)
		}
	}
	return nil
}

func firstNonEmptyBootstrap(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// Start launches the RPC server and background runtimes.
func (a *App) Start(ctx context.Context) error {
	return a.server.Start(ctx)
}

func (a *App) Close() error {
	if a.playwright != nil {
		_ = a.playwright.Stop()
	}
	if a.ocr != nil {
		_ = a.ocr.Stop()
	}
	if a.media != nil {
		_ = a.media.Stop()
	}
	if a.storage == nil {
		return nil
	}

	return a.storage.Close()
}

// chooseRuntimeOnStart keeps a runtime instance after Start fails so the shared
// plugin runtime cache preserves the concrete failure state instead of being
// overwritten by a generic unavailable placeholder. Constructor failures may
// still return a non-nil runtime shell that carries the concrete failure state.
func chooseRuntimeOnStart[T runtimeStarter](runtime T, buildErr error, unavailable func() T) T {
	if buildErr != nil {
		value := reflect.ValueOf(runtime)
		if value.IsValid() && !(value.Kind() == reflect.Ptr && value.IsNil()) {
			return runtime
		}
		return unavailable()
	}
	if err := runtime.Start(); err != nil {
		return runtime
	}
	return runtime
}
