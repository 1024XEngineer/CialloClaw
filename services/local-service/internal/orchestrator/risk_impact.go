package orchestrator

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/delivery"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
)

func deriveImpactScopeFiles(task runengine.TaskRecord, pendingExecution map[string]any, deliveryService *delivery.Service) []string {
	files := make([]string, 0, 4)
	files = appendImpactScopePath(files, stringValue(task.StorageWritePlan, "target_path", ""))
	for _, artifactPlan := range task.ArtifactPlans {
		files = appendImpactScopePath(files, stringValue(artifactPlan, "path", ""))
	}
	files = appendImpactScopePath(files, pathFromDeliveryResult(task.DeliveryResult))
	files = appendImpactScopePath(files, pathFromPendingExecution(task.TaskID, pendingExecution, deliveryService))
	files = appendImpactScopePath(files, targetPathFromIntent(task.Intent))
	return files
}

func appendImpactScopePath(files []string, candidate string) []string {
	candidate = strings.TrimSpace(strings.ReplaceAll(candidate, "\\", "/"))
	if candidate == "" {
		return files
	}
	candidate = path.Clean(candidate)
	if candidate == "." {
		return files
	}
	for _, existing := range files {
		if existing == candidate {
			return files
		}
	}
	return append(files, candidate)
}

func pathFromPendingExecution(taskID string, pendingExecution map[string]any, deliveryService *delivery.Service) string {
	if len(pendingExecution) == 0 {
		return ""
	}
	deliveryType := stringValue(pendingExecution, "delivery_type", "")
	if deliveryType != "workspace_document" {
		return ""
	}
	resultTitle := stringValue(pendingExecution, "result_title", "处理结果")
	previewText := stringValue(pendingExecution, "preview_text", "")
	deliveryResult := deliveryService.BuildDeliveryResult(taskID, deliveryType, resultTitle, previewText)
	return pathFromDeliveryResult(deliveryResult)
}

func pathFromDeliveryResult(deliveryResult map[string]any) string {
	payload, ok := deliveryResult["payload"].(map[string]any)
	if !ok {
		return ""
	}
	return stringValue(payload, "path", "")
}

func targetPathFromIntent(taskIntent map[string]any) string {
	targetPath := stringValue(mapValue(taskIntent, "arguments"), "target_path", "")
	switch targetPath {
	case "", "workspace_document", "bubble", "result_page", "task_detail", "open_file", "reveal_in_folder":
		return ""
	default:
		return targetPath
	}
}

func isWorkspaceRelativePath(filePath, workspaceRoot string) bool {
	trimmedPath := strings.TrimSpace(filePath)
	if trimmedPath == "" {
		return false
	}
	if hasWindowsDriveLetterPrefix(trimmedPath) {
		if !isWindowsStyleAbsolutePath(trimmedPath) {
			return false
		}
	}
	if !filepath.IsAbs(trimmedPath) && !isWindowsStyleAbsolutePath(trimmedPath) {
		if strings.HasPrefix(trimmedPath, "\\") || strings.HasPrefix(trimmedPath, "/") {
			return false
		}
	}
	normalizedPath := strings.Trim(strings.ReplaceAll(filePath, "\\", "/"), "/")
	if normalizedPath == "" {
		return false
	}
	if normalizedPath == "workspace" || strings.HasPrefix(normalizedPath, "workspace/") {
		return true
	}
	if filepath.IsAbs(trimmedPath) || isWindowsStyleAbsolutePath(trimmedPath) {
		cleanRoot := filepath.Clean(strings.TrimSpace(workspaceRoot))
		if cleanRoot == "" {
			return false
		}
		cleanPath := filepath.Clean(trimmedPath)
		rootWithSeparator := cleanRoot + string(filepath.Separator)
		return cleanPath == cleanRoot || strings.HasPrefix(cleanPath, rootWithSeparator)
	}
	cleanRelative := path.Clean(normalizedPath)
	// Runtime temp artifacts remain openable from the desktop host, but governance
	// must not classify them as workspace-contained when computing trust scope.
	if cleanRelative == "temp" || strings.HasPrefix(cleanRelative, "temp/") {
		return false
	}
	return cleanRelative != ".." && !strings.HasPrefix(cleanRelative, "../")
}

func hasWindowsDriveLetterPrefix(value string) bool {
	if len(value) < 2 {
		return false
	}
	letter := value[0]
	return ((letter >= 'A' && letter <= 'Z') || (letter >= 'a' && letter <= 'z')) && value[1] == ':'
}

func isWindowsStyleAbsolutePath(value string) bool {
	return hasWindowsDriveLetterPrefix(value) && len(value) >= 3 && (value[2] == '\\' || value[2] == '/')
}

func hasOverwriteOrDeleteRisk(taskIntent map[string]any) bool {
	if stringValue(taskIntent, "name", "") == "write_file" {
		return true
	}
	arguments := mapValue(taskIntent, "arguments")
	return boolValue(arguments, "overwrite", false) || boolValue(arguments, "delete", false)
}

// buildImpactScope derives the minimal impact summary used by authorization
// results and the security views. It intentionally normalizes files around the
// workspace root so policy, audit, and restore flows all reason about one scope
// shape instead of transport- or tool-specific paths.
func (s *Service) buildImpactScope(task runengine.TaskRecord, pendingExecution map[string]any) map[string]any {
	if impactScope, ok := pendingExecution["impact_scope"].(map[string]any); ok && len(impactScope) > 0 {
		return cloneMap(impactScope)
	}
	files := deriveImpactScopeFiles(task, pendingExecution, s.delivery)
	workspacePath := currentRuntimeWorkspaceRoot(s.executor)
	outOfWorkspace := false
	for _, filePath := range files {
		if !isWorkspaceRelativePath(filePath, workspacePath) {
			outOfWorkspace = true
			break
		}
	}

	return map[string]any{
		"files":                    files,
		"webpages":                 []string{},
		"apps":                     []string{},
		"out_of_workspace":         outOfWorkspace,
		"overwrite_or_delete_risk": hasOverwriteOrDeleteRisk(task.Intent),
	}
}
