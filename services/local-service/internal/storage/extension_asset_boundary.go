package storage

import "strings"

const (
	extensionAssetSourceBuiltin     = "builtin"
	extensionAssetSourceLocalDir    = "local_dir"
	extensionAssetSourceGitHub      = "github"
	extensionAssetSourceMarketplace = "marketplace"
)

// NormalizeExtensionAssetReferences filters extension asset references down to
// the current owner-5 formal boundary so execution/trace/eval cannot silently
// expand new asset kinds or unsupported source types ahead of roadmap approval.
func NormalizeExtensionAssetReferences(items []ExtensionAssetReference) []ExtensionAssetReference {
	if len(items) == 0 {
		return nil
	}
	result := make([]ExtensionAssetReference, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		normalized, ok := normalizeExtensionAssetReference(item)
		if !ok {
			continue
		}
		key := strings.Join([]string{normalized.AssetKind, normalized.AssetID, normalized.Version, normalized.Source}, "|")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, normalized)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeExtensionAssetReference(item ExtensionAssetReference) (ExtensionAssetReference, bool) {
	normalized := ExtensionAssetReference{
		AssetKind:    strings.TrimSpace(item.AssetKind),
		AssetID:      strings.TrimSpace(item.AssetID),
		Name:         strings.TrimSpace(item.Name),
		Version:      strings.TrimSpace(item.Version),
		Source:       strings.TrimSpace(item.Source),
		Summary:      strings.TrimSpace(item.Summary),
		Entry:        strings.TrimSpace(item.Entry),
		Capabilities: normalizeExtensionAssetStringList(item.Capabilities),
		Permissions:  normalizeExtensionAssetStringList(item.Permissions),
		RuntimeNames: normalizeExtensionAssetStringList(item.RuntimeNames),
	}
	if normalized.AssetKind == "" || normalized.AssetID == "" || normalized.Version == "" || normalized.Source == "" {
		return ExtensionAssetReference{}, false
	}
	switch normalized.AssetKind {
	case ExtensionAssetKindSkillManifest, ExtensionAssetKindBlueprintDefinition, ExtensionAssetKindPromptTemplateVersion:
		if normalized.Source != extensionAssetSourceBuiltin {
			return ExtensionAssetReference{}, false
		}
		normalized.Entry = ""
		normalized.Capabilities = nil
		normalized.Permissions = nil
		normalized.RuntimeNames = nil
	case ExtensionAssetKindPluginManifest:
		if !allowedPluginManifestSource(normalized.Source) {
			return ExtensionAssetReference{}, false
		}
	case ExtensionAssetKindModelProviderRoute, ExtensionAssetKindPerceptionPackage:
		if normalized.Source != extensionAssetSourceBuiltin {
			return ExtensionAssetReference{}, false
		}
		normalized.RuntimeNames = nil
	default:
		return ExtensionAssetReference{}, false
	}
	return normalized, true
}

func normalizeExtensionAssetStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		candidate := strings.TrimSpace(value)
		if candidate == "" {
			continue
		}
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		result = append(result, candidate)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func allowedPluginManifestSource(source string) bool {
	switch source {
	case extensionAssetSourceBuiltin, extensionAssetSourceLocalDir, extensionAssetSourceGitHub, extensionAssetSourceMarketplace:
		return true
	default:
		return false
	}
}
