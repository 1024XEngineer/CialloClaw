package execution

import (
	"strings"

	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/perception"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

// supplementalExecutionBoundaryAssets attributes model-provider and perception
// package boundaries only when the current task actually used those subsystems.
func supplementalExecutionBoundaryAssets(request Request, result Result, modelService *model.Service) []storage.ExtensionAssetReference {
	refs := make([]storage.ExtensionAssetReference, 0, 2)
	if providerRef, ok := modelProviderRouteRef(modelService, result); ok {
		refs = append(refs, providerRef)
	}
	if perceptionRef, ok := perceptionPackageRef(request); ok {
		refs = append(refs, perceptionRef)
	}
	return refs
}

func modelProviderRouteRef(modelService *model.Service, result Result) (storage.ExtensionAssetReference, bool) {
	if modelService == nil || len(result.ModelInvocation) == 0 {
		return storage.ExtensionAssetReference{}, false
	}
	invocationProvider := strings.TrimSpace(stringValue(result.ModelInvocation, "provider", ""))
	if invocationProvider == "" || invocationProvider != strings.TrimSpace(modelService.Provider()) {
		return storage.ExtensionAssetReference{}, false
	}
	descriptor, ok := model.LookupProviderDescriptor(modelService.Provider())
	if !ok || strings.TrimSpace(descriptor.Name) == "" {
		return storage.ExtensionAssetReference{}, false
	}
	return storage.ExtensionAssetReference{
		AssetKind:    storage.ExtensionAssetKindModelProviderRoute,
		AssetID:      descriptor.Name,
		Name:         firstNonEmptyExecution(strings.TrimSpace(descriptor.DisplayName), strings.TrimSpace(descriptor.Name)),
		Version:      strings.TrimSpace(descriptor.Version),
		Source:       strings.TrimSpace(descriptor.Source),
		Summary:      strings.TrimSpace(descriptor.Summary),
		Entry:        strings.TrimSpace(descriptor.Entry),
		Capabilities: append([]string(nil), descriptor.Capabilities...),
		Permissions:  append([]string(nil), descriptor.Permissions...),
	}, true
}

func perceptionPackageRef(request Request) (storage.ExtensionAssetReference, bool) {
	if !snapshotUsesPerceptionBoundary(request.Snapshot) {
		return storage.ExtensionAssetReference{}, false
	}
	descriptor := perception.DefaultPackageDescriptor()
	if strings.TrimSpace(descriptor.PackageID) == "" {
		return storage.ExtensionAssetReference{}, false
	}
	return storage.ExtensionAssetReference{
		AssetKind:    storage.ExtensionAssetKindPerceptionPackage,
		AssetID:      descriptor.PackageID,
		Name:         strings.TrimSpace(descriptor.Name),
		Version:      strings.TrimSpace(descriptor.Version),
		Source:       strings.TrimSpace(descriptor.Source),
		Summary:      strings.TrimSpace(descriptor.Summary),
		Entry:        strings.TrimSpace(descriptor.Entry),
		Capabilities: append([]string(nil), descriptor.Capabilities...),
		Permissions:  append([]string(nil), descriptor.Permissions...),
	}, true
}

func snapshotUsesPerceptionBoundary(snapshot contextsvc.TaskContextSnapshot) bool {
	return strings.TrimSpace(snapshot.SelectionText) != "" ||
		strings.TrimSpace(snapshot.ErrorText) != "" ||
		strings.TrimSpace(snapshot.PageTitle) != "" ||
		strings.TrimSpace(snapshot.PageURL) != "" ||
		strings.TrimSpace(snapshot.AppName) != "" ||
		strings.TrimSpace(snapshot.WindowTitle) != "" ||
		strings.TrimSpace(snapshot.VisibleText) != "" ||
		strings.TrimSpace(snapshot.ScreenSummary) != "" ||
		strings.TrimSpace(snapshot.ClipboardText) != "" ||
		strings.TrimSpace(snapshot.HoverTarget) != "" ||
		strings.TrimSpace(snapshot.LastAction) != "" ||
		snapshot.DwellMillis > 0 ||
		snapshot.CopyCount > 0 ||
		snapshot.WindowSwitches > 0 ||
		snapshot.PageSwitches > 0
}

func firstNonEmptyExecution(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
