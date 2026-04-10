package builtin

import (
	"path"
	"path/filepath"
	"strings"
)

func normalizeWorkspaceToolPath(pathValue string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(pathValue, "\\", "/"))
	if normalized == "" {
		return ""
	}

	if isToolAbsolutePath(normalized) {
		return path.Clean(normalized)
	}

	normalized = strings.TrimPrefix(normalized, "./")
	if normalized == "workspace" {
		return "."
	}
	if strings.HasPrefix(normalized, "workspace/") {
		normalized = strings.TrimPrefix(normalized, "workspace/")
	}

	cleaned := path.Clean(normalized)
	if cleaned == "." {
		return "."
	}
	if strings.HasPrefix(cleaned, "../") {
		return ""
	}
	return cleaned
}

func isToolAbsolutePath(pathValue string) bool {
	if filepath.IsAbs(pathValue) {
		return true
	}
	return len(pathValue) >= 3 && pathValue[1] == ':' && (pathValue[2] == '/' || pathValue[2] == '\\')
}
