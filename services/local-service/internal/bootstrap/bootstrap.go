// Package bootstrap assembles local-service dependencies and startup wiring.
package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/rpc"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/builtin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/sidecarclient"
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
	getExecutablePathForBootstrap         = os.Executable
)

type runtimeMigrationPlan struct {
	legacyWorkspaceRoot string
	targetWorkspaceRoot string
	legacyDatabasePath  string
	targetDatabasePath  string
	legacySecretPath    string
	targetSecretPath    string
}

type sidecarRuntimes struct {
	playwright *sidecarclient.PlaywrightSidecarRuntime
	ocr        *sidecarclient.OCRWorkerRuntime
	media      *sidecarclient.MediaWorkerRuntime
}

// New assembles a fully wired local-service application.
func New(cfg config.Config) (*App, error) {
	if strings.ContainsRune(cfg.WorkspaceRoot, '\x00') {
		return nil, fmt.Errorf("workspace root contains invalid null byte")
	}
	if err := migrateLegacyRuntimeDefaultsIfNeeded(cfg, legacyRuntimeRootsForCompatibility()); err != nil {
		return nil, err
	}
	core, err := buildCoreDeps(cfg)
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			_ = core.storageService.Close()
		}
	}()

	runtimes, err := buildRuntimes(core)
	if err != nil {
		return nil, err
	}

	services, err := buildServices(core, runtimes)
	if err != nil {
		return nil, err
	}

	success = true
	return newApp(cfg, core, runtimes, services), nil
}

// buildSidecarRuntimes keeps worker startup policy in one bootstrap phase. A
// runtime may start in an unavailable state, but callers still receive a stable
// client facade so tool wiring can preserve the formal tool_call flow.
func buildSidecarRuntimes(pluginService *plugin.Service, osCapability platform.OSCapabilityAdapter) sidecarRuntimes {
	playwrightRuntime, err := sidecarclient.NewPlaywrightSidecarRuntime(pluginService, osCapability)
	playwrightRuntime = chooseRuntimeOnStart(playwrightRuntime, err, func() *sidecarclient.PlaywrightSidecarRuntime {
		return sidecarclient.NewUnavailablePlaywrightSidecarRuntime(pluginService, osCapability)
	})
	ocrRuntime, err := sidecarclient.NewOCRWorkerRuntime(pluginService, osCapability)
	ocrRuntime = chooseRuntimeOnStart(ocrRuntime, err, func() *sidecarclient.OCRWorkerRuntime {
		return sidecarclient.NewUnavailableOCRWorkerRuntime(pluginService, osCapability)
	})
	mediaRuntime, err := sidecarclient.NewMediaWorkerRuntime(pluginService, osCapability)
	mediaRuntime = chooseRuntimeOnStart(mediaRuntime, err, func() *sidecarclient.MediaWorkerRuntime {
		return sidecarclient.NewUnavailableMediaWorkerRuntime(pluginService, osCapability)
	})
	return sidecarRuntimes{
		playwright: playwrightRuntime,
		ocr:        ocrRuntime,
		media:      mediaRuntime,
	}
}

// migrateLegacyRuntimeDefaultsIfNeeded copies data from the legacy repo-relative
// runtime layout into the new per-user runtime root before storage opens. This
// preserves task history and settings for upgrades that previously relied on
// relative defaults like ./workspace and ./data/cialloclaw.db.
func migrateLegacyRuntimeDefaultsIfNeeded(cfg config.Config, legacyRoots []string) error {
	plan, ok := buildRuntimeMigrationPlan(cfg, legacyRoots)
	if !ok {
		return nil
	}
	if err := copyDirectoryIfMissing(plan.legacyWorkspaceRoot, plan.targetWorkspaceRoot); err != nil {
		return err
	}
	if err := copyFileIfMissing(plan.legacyDatabasePath, plan.targetDatabasePath); err != nil {
		return err
	}
	if err := copyFileIfMissing(plan.legacySecretPath, plan.targetSecretPath); err != nil {
		return err
	}
	return nil
}

func buildRuntimeMigrationPlan(cfg config.Config, legacyRoots []string) (runtimeMigrationPlan, bool) {
	targetWorkspaceRoot := filepath.Clean(cfg.WorkspaceRoot)
	targetDatabasePath := filepath.Clean(cfg.DatabasePath)
	if targetWorkspaceRoot != filepath.Clean(config.DefaultWorkspaceRoot()) || targetDatabasePath != filepath.Clean(config.DefaultDatabasePath()) {
		return runtimeMigrationPlan{}, false
	}
	for _, legacyRoot := range legacyRoots {
		trimmedRoot := strings.TrimSpace(legacyRoot)
		if trimmedRoot == "" {
			continue
		}
		legacyWorkspaceRoot := filepath.Join(trimmedRoot, "workspace")
		legacyDatabasePath := filepath.Join(trimmedRoot, "data", "cialloclaw.db")
		legacySecretPath := secretStorePathForDatabase(legacyDatabasePath)
		if sameFilePath(legacyWorkspaceRoot, targetWorkspaceRoot) || sameFilePath(legacyDatabasePath, targetDatabasePath) {
			continue
		}
		if !pathExists(legacyWorkspaceRoot) && !pathExists(legacyDatabasePath) && !pathExists(legacySecretPath) {
			continue
		}
		return runtimeMigrationPlan{
			legacyWorkspaceRoot: legacyWorkspaceRoot,
			targetWorkspaceRoot: targetWorkspaceRoot,
			legacyDatabasePath:  legacyDatabasePath,
			targetDatabasePath:  targetDatabasePath,
			legacySecretPath:    legacySecretPath,
			targetSecretPath:    secretStorePathForDatabase(targetDatabasePath),
		}, true
	}
	return runtimeMigrationPlan{}, false
}

// legacyRuntimeRootsForCompatibility trusts only the executable-adjacent legacy
// layout because the process working directory is not a stable bootstrap trust
// boundary in packaged builds.
func legacyRuntimeRootsForCompatibility() []string {
	if executablePath, err := getExecutablePathForBootstrap(); err == nil {
		return dedupePaths([]string{filepath.Dir(executablePath)})
	}
	return nil
}

func dedupePaths(paths []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(paths))
	for _, pathValue := range paths {
		trimmed := strings.TrimSpace(pathValue)
		if trimmed == "" {
			continue
		}
		cleaned := filepath.Clean(trimmed)
		key := filepath.ToSlash(cleaned)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, cleaned)
	}
	return result
}

func sameFilePath(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}

func pathExists(pathValue string) bool {
	_, err := os.Stat(pathValue)
	return err == nil
}

func secretStorePathForDatabase(databasePath string) string {
	trimmed := strings.TrimSpace(databasePath)
	if trimmed == "" {
		return ""
	}
	ext := filepath.Ext(trimmed)
	if ext == "" {
		return trimmed + ".stronghold.db"
	}
	return strings.TrimSuffix(trimmed, ext) + ".stronghold" + ext
}

func copyDirectoryIfMissing(sourceRoot, targetRoot string) error {
	if !pathExists(sourceRoot) {
		return nil
	}
	return filepath.WalkDir(sourceRoot, func(currentPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relativePath, err := filepath.Rel(sourceRoot, currentPath)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(targetRoot, relativePath)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		return copyFileContents(currentPath, targetPath, entry.Type())
	})
}

func copyFileIfMissing(sourcePath, targetPath string) error {
	if !pathExists(sourcePath) || pathExists(targetPath) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	return copyFileContents(sourcePath, targetPath, 0)
}

func copyFileContents(sourcePath, targetPath string, entryMode os.FileMode) error {
	reader, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()
	mode := os.FileMode(0o644)
	if entryMode != 0 {
		mode = entryMode.Perm()
		if mode == 0 {
			mode = 0o644
		}
	}
	writer, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		// Legacy runtime migration must stay idempotent across repeated launches, so
		// pre-existing destination files are treated as already migrated content.
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return err
	}
	defer func() { _ = writer.Close() }()
	_, err = io.Copy(writer, reader)
	return err
}

func loadBootstrapModelConfig(base config.ModelConfig, settingsStore storage.SettingsStore) (config.ModelConfig, config.ModelConfig, bool, error) {
	if settingsStore == nil {
		return base, base, false, nil
	}
	snapshot, err := settingsStore.LoadSettingsSnapshot(context.Background())
	if err != nil {
		return config.ModelConfig{}, config.ModelConfig{}, false, err
	}
	resolved := model.RuntimeConfigFromSettings(base, snapshot)
	placeholder := resolved
	if provider := bootstrapPersistedModelProvider(snapshot); provider != "" {
		placeholder.Provider = provider
	}
	persistedRouteChanged := placeholder.Provider != base.Provider || placeholder.Endpoint != base.Endpoint || placeholder.ModelID != base.ModelID
	return resolved, placeholder, persistedRouteChanged, nil
}

func bootstrapPersistedModelProvider(snapshot map[string]any) string {
	models, ok := snapshot["models"].(map[string]any)
	if !ok {
		return ""
	}
	if provider, ok := models["provider"].(string); ok {
		if trimmed := strings.TrimSpace(provider); trimmed != "" {
			return trimmed
		}
	}
	credentials, ok := models["credentials"].(map[string]any)
	if !ok {
		return ""
	}
	provider, _ := credentials["provider"].(string)
	return strings.TrimSpace(provider)
}

func shouldFallbackBootstrapModelService(err error, allowPersistedRoutePlaceholder bool) bool {
	if allowPersistedRoutePlaceholder && errors.Is(err, model.ErrModelProviderUnsupported) {
		return true
	}
	if !errors.Is(err, model.ErrSecretSourceFailed) {
		return false
	}
	return errors.Is(err, model.ErrSecretNotFound) ||
		errors.Is(err, storage.ErrSecretNotFound) ||
		errors.Is(err, storage.ErrSecretStoreAccessFailed) ||
		errors.Is(err, storage.ErrStrongholdUnavailable) ||
		errors.Is(err, storage.ErrStrongholdAccessFailed)
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
