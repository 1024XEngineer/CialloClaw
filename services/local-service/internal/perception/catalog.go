package perception

import "strings"

// PackageDescriptor reserves the future perception-package boundary without
// turning on marketplace or installation workflows before the roadmap allows it.
type PackageDescriptor struct {
	PackageID    string
	Name         string
	Version      string
	Source       string
	Entry        string
	Summary      string
	Capabilities []string
	Permissions  []string
	Scenes       []string
}

var builtinPackageCatalog = []PackageDescriptor{{
	PackageID:    "desktop_context_core",
	Name:         "Desktop Context Core",
	Version:      "builtin-v1",
	Source:       "builtin",
	Entry:        "builtin://perception-package/desktop_context_core",
	Summary:      "Built-in perception package for screen, page, clipboard, and behavior context signals.",
	Capabilities: []string{"page_context", "screen_context", "clipboard_context", "behavior_signals"},
	Permissions:  []string{"screen:read", "clipboard:read", "page_context:read", "behavior:read"},
	Scenes:       []string{"hover", "selected_text", "dashboard", "task_runtime"},
}}

// BuiltinPackageDescriptors returns the stable builtin perception package catalog.
func BuiltinPackageDescriptors() []PackageDescriptor {
	result := make([]PackageDescriptor, 0, len(builtinPackageCatalog))
	for _, item := range builtinPackageCatalog {
		result = append(result, clonePackageDescriptor(item))
	}
	return result
}

// LookupPackageDescriptor resolves one builtin perception package descriptor.
func LookupPackageDescriptor(packageID string) (PackageDescriptor, bool) {
	needle := strings.TrimSpace(packageID)
	for _, item := range builtinPackageCatalog {
		if item.PackageID == needle {
			return clonePackageDescriptor(item), true
		}
	}
	return PackageDescriptor{}, false
}

// DefaultPackageDescriptor returns the current builtin perception package that
// execution can attribute when one task actually consumed perception signals.
func DefaultPackageDescriptor() PackageDescriptor {
	if len(builtinPackageCatalog) == 0 {
		return PackageDescriptor{}
	}
	return clonePackageDescriptor(builtinPackageCatalog[0])
}

func clonePackageDescriptor(item PackageDescriptor) PackageDescriptor {
	item.Capabilities = append([]string(nil), item.Capabilities...)
	item.Permissions = append([]string(nil), item.Permissions...)
	item.Scenes = append([]string(nil), item.Scenes...)
	return item
}
