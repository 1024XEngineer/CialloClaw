package execution

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"path"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/presentation"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func (s *Service) availableToolNames() []string {
	if s.tools == nil {
		return nil
	}
	return s.tools.Names()
}

func (s *Service) availableWorkers() []string {
	if s.plugin == nil {
		return nil
	}
	return s.plugin.Workers()
}

func (s *Service) executeTool(ctx context.Context, request Request, workspacePath, toolName string, input map[string]any) (*tools.ToolExecutionResult, map[string]any, error) {
	if s.executor == nil {
		return nil, nil, fmt.Errorf("tool executor is required")
	}
	execCtx := s.toolExecutionContext(workspacePath, request, toolName)
	recoveryPoint, err := s.prepareGovernanceRecoveryPoint(ctx, request, workspacePath, toolName, input)
	if err != nil {
		return nil, cloneMap(recoveryPoint), err
	}
	toolResult, err := s.executor.ExecuteToolWithContext(ctx, execCtx, toolName, input)
	if toolResult != nil && len(recoveryPoint) > 0 {
		if toolResult.RawOutput == nil {
			toolResult.RawOutput = map[string]any{}
		}
		toolResult.RawOutput["recovery_point"] = cloneMap(recoveryPoint)
	}
	return toolResult, cloneMap(recoveryPoint), err
}

func (s *Service) resolveGovernanceToolExecution(request Request) (string, map[string]any, *tools.ToolExecuteContext, bool, error) {
	intentName := stringValue(request.Intent, "name", "")
	args := mapValue(request.Intent, "arguments")
	deliveryType := firstNonEmpty(strings.TrimSpace(request.DeliveryType), "workspace_document")
	previewText := previewTextForDeliveryType(deliveryType)
	deliveryResult := s.delivery.BuildDeliveryResultWithTargetPath(
		request.TaskID,
		deliveryType,
		firstNonEmpty(strings.TrimSpace(request.ResultTitle), presentation.Text(presentation.MessageResultTitleGeneric, nil)),
		previewText,
		targetPathFromIntent(request.Intent),
	)
	if s.tools != nil && intentName != "" && intentName != "write_file" {
		if _, err := s.tools.Get(intentName); err == nil {
			if budgetDowngradeDisallowsDirectTool(request, intentName) {
				return "", nil, nil, false, nil
			}
			if input, ok := resolveBrowserToolInput(intentName, args, request.Snapshot); ok {
				return intentName, input, s.toolExecutionContext(s.workspace, request, intentName), true, nil
			}
			if input, ok := resolveDirectToolInput(intentName, args, request.Snapshot); ok {
				return intentName, input, s.toolExecutionContext(s.workspace, request, intentName), true, nil
			}
		}
	}
	rawTargetPath := firstNonEmpty(targetPathFromIntent(request.Intent), deliveryPayloadPath(deliveryResult))
	writePath := workspaceFSPath(rawTargetPath)
	if writePath == "" {
		writePath = strings.TrimSpace(strings.ReplaceAll(rawTargetPath, "\\", "/"))
	}
	if writePath == "" {
		return "", nil, nil, false, nil
	}
	toolName, toolInput := "write_file", map[string]any{"path": writePath, "content": ""}
	return toolName, toolInput, s.toolExecutionContext(s.workspace, request, toolName), true, nil
}

func (s *Service) toolExecutionContext(workspacePath string, request Request, toolName string) *tools.ToolExecuteContext {
	workspacePath = firstNonEmpty(strings.TrimSpace(workspacePath), s.workspace)
	approvedOperation := firstNonEmpty(strings.TrimSpace(request.ApprovedOperation), stringValue(request.Intent, "name", ""))
	approvedTargetObject := firstNonEmpty(strings.TrimSpace(request.ApprovedTargetObject), approvedTargetObject(request.Intent, s.workspace))
	modelService := s.currentModel()
	platformAdapter := firstNonNilFileSystem(s.toolPlatform, s.fileSystem)
	if strings.TrimSpace(toolName) == "write_file" {
		// write_file stays pinned to the real workspace adapter so broader
		// desktop-folder read access does not silently widen mutation governance
		// or bypass recovery-point coverage.
		platformAdapter = s.fileSystem
	}
	return &tools.ToolExecuteContext{
		TaskID:               request.TaskID,
		RunID:                request.RunID,
		WorkspacePath:        workspacePath,
		ApprovalGranted:      request.ApprovalGranted,
		ApprovedOperation:    approvedOperation,
		ApprovedTargetObject: approvedTargetObject,
		ApprovedToolInput:    cloneMap(request.ApprovedToolInput),
		Platform:             platformAdapter,
		Execution:            s.execution,
		Playwright:           s.playwright,
		OCR:                  s.ocr,
		Media:                s.media,
		Model:                modelService,
	}
}

func firstNonNilFileSystem(primary, fallback platform.FileSystemAdapter) platform.FileSystemAdapter {
	if primary != nil {
		return primary
	}
	return fallback
}

func (s *Service) prepareGovernanceRecoveryPoint(ctx context.Context, request Request, workspacePath, toolName string, input map[string]any) (map[string]any, error) {
	if s.checkpoint == nil {
		return nil, nil
	}
	switch toolName {
	case "write_file":
		if s.fileSystem == nil {
			return nil, nil
		}
		targetPath := stringValue(input, "path", "")
		if targetPath == "" {
			return nil, nil
		}
		if _, err := s.fileSystem.EnsureWithinWorkspace(targetPath); err != nil {
			return nil, nil
		}
		point, err := s.checkpoint.CreateWithSnapshots(ctx, s.fileSystem, checkpoint.CreateInput{
			TaskID:  request.TaskID,
			Summary: "write_file_before_change",
			Objects: []string{checkpointObjectPath(targetPath)},
		})
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrRecoveryPointPrepareFailed, err)
		}
		return recoveryPointMap(point), nil
	case "exec_command":
		return nil, nil
	default:
		return nil, nil
	}
}

func recoveryPointMap(point checkpoint.RecoveryPoint) map[string]any {
	return map[string]any{
		"recovery_point_id": point.RecoveryPointID,
		"task_id":           point.TaskID,
		"summary":           point.Summary,
		"created_at":        point.CreatedAt,
		"objects":           append([]string(nil), point.Objects...),
	}
}

// GovernanceTargetObject derives the stable approval boundary for a concrete
// tool invocation. Orchestrator runtime-approval recovery reuses the same
// target-object rules so resumed approval checks match the executor's
// preflight and replay behavior.
func GovernanceTargetObject(toolName string, toolInput map[string]any, execCtx *tools.ToolExecuteContext) string {
	switch toolName {
	case "write_file":
		return stringValue(toolInput, "path", "")
	case "exec_command":
		if execCtx == nil {
			return stringValue(toolInput, "working_dir", "")
		}
		return firstNonEmpty(stringValue(toolInput, "working_dir", ""), execCtx.WorkspacePath)
	case "page_read", "page_search", "page_interact", "web_search":
		return stringValue(toolInput, "url", "")
	case "browser_navigate":
		return firstNonEmpty(strings.TrimSpace(stringValue(toolInput, "url", "")), browserStableTargetObject(mapValue(toolInput, "attach")))
	case "browser_tab_focus", "browser_interact":
		return browserStableTargetObject(mapValue(toolInput, "attach"))
	case "browser_attach_current", "browser_snapshot", "browser_tabs_list":
		return browserTargetObject(mapValue(toolInput, "attach"))
	default:
		for _, key := range governedTargetKeys(toolName) {
			if value := stringValue(toolInput, key, ""); value != "" {
				return value
			}
		}
		return ""
	}
}

func governanceTargetObject(toolName string, toolInput map[string]any, execCtx *tools.ToolExecuteContext) string {
	return GovernanceTargetObject(toolName, toolInput, execCtx)
}

func approvedTargetObject(intent map[string]any, workspacePath string) string {
	intentName := stringValue(intent, "name", "")
	arguments := mapValue(intent, "arguments")
	if browserTarget := browserIntentTargetObject(intentName, arguments); browserTarget != "" {
		return browserTarget
	}
	for _, key := range approvedTargetKeys(intentName) {
		if value := strings.TrimSpace(stringValue(arguments, key, "")); value != "" {
			normalized := strings.ReplaceAll(value, "\\", "/")
			if key != "working_dir" {
				if candidate := workspaceFSPath(normalized); candidate != "" {
					normalized = candidate
				}
			}
			workspaceRoot := strings.ReplaceAll(strings.TrimSpace(workspacePath), "\\", "/")
			if workspaceRoot != "" && !path.IsAbs(normalized) && !isWindowsAbsolutePath(normalized) {
				return path.Join(workspaceRoot, normalized)
			}
			return normalized
		}
	}
	if intentName == "exec_command" {
		return workspacePath
	}
	if intentName == "browser_navigate" {
		if url := strings.TrimSpace(stringValue(arguments, "url", "")); url != "" {
			return url
		}
	}
	if target := browserIntentTargetObject(intentName, arguments); target != "" {
		return target
	}
	if url := strings.TrimSpace(stringValue(arguments, "url", "")); url != "" {
		return url
	}
	return ""
}

func governedTargetKeys(toolName string) []string {
	switch strings.TrimSpace(toolName) {
	case "transcode_media", "normalize_recording":
		return []string{"output_path", "path"}
	case "extract_frames":
		return []string{"output_dir", "path"}
	default:
		return []string{"path", "target_path", "file_path"}
	}
}

func approvedTargetKeys(intentName string) []string {
	switch strings.TrimSpace(intentName) {
	case "transcode_media", "normalize_recording":
		return []string{"output_path", "target_path", "path", "working_dir"}
	case "extract_frames":
		return []string{"output_dir", "target_path", "path", "working_dir"}
	default:
		return []string{"target_path", "path", "working_dir"}
	}
}

func resolveBrowserToolInput(intentName string, arguments map[string]any, snapshot taskcontext.TaskContextSnapshot) (map[string]any, bool) {
	if explicitInput, ok := resolveExplicitBrowserToolInput(intentName, arguments); ok {
		return explicitInput, true
	}

	browserKind := strings.ToLower(strings.TrimSpace(snapshot.BrowserKind))
	if browserKind != "chrome" && browserKind != "edge" {
		return nil, false
	}

	useSnapshotTarget := true
	allowEmptyTarget := false
	requireStableTarget := requiresStableBrowserTarget(intentName)
	if intentName == "browser_tabs_list" {
		allowEmptyTarget = true
	}
	if intentName == "browser_tab_focus" {
		useSnapshotTarget = browserTargetOverrideMissing(arguments)
	}

	attach := buildBrowserAttachInput(browserKind, snapshot, arguments, useSnapshotTarget, allowEmptyTarget, requireStableTarget)
	if len(attach) == 0 {
		return nil, false
	}

	input := map[string]any{"attach": attach}
	switch strings.TrimSpace(intentName) {
	case "browser_attach_current", "browser_snapshot", "browser_tabs_list", "browser_tab_focus":
		return input, true
	case "browser_navigate":
		urlValue := strings.TrimSpace(stringValue(arguments, "url", ""))
		if urlValue == "" {
			return nil, false
		}
		input["url"] = urlValue
		return input, true
	case "browser_interact":
		actions, ok := arguments["actions"]
		if !ok {
			return nil, false
		}
		input["actions"] = actions
		return input, true
	default:
		return nil, false
	}
}

func resolveExplicitBrowserToolInput(intentName string, arguments map[string]any) (map[string]any, bool) {
	attach := mergeExplicitBrowserAttachInput(mapValue(arguments, "attach"), arguments)
	if len(attach) == 0 {
		return nil, false
	}
	if requiresStableBrowserTarget(intentName) && !hasStableBrowserAttachTarget(attach) {
		return nil, false
	}

	input := map[string]any{"attach": cloneMap(attach)}
	switch strings.TrimSpace(intentName) {
	case "browser_attach_current", "browser_snapshot", "browser_tabs_list", "browser_tab_focus":
		return input, true
	case "browser_navigate":
		urlValue := strings.TrimSpace(stringValue(arguments, "url", ""))
		if urlValue == "" {
			return nil, false
		}
		input["url"] = urlValue
		return input, true
	case "browser_interact":
		actions, ok := arguments["actions"]
		if !ok {
			return nil, false
		}
		input["actions"] = actions
		return input, true
	default:
		return nil, false
	}
}

func mergeExplicitBrowserAttachInput(attachInput map[string]any, arguments map[string]any) map[string]any {
	merged := cloneMap(attachInput)
	if merged == nil {
		merged = map[string]any{}
	}
	target := cloneMap(mapValue(merged, "target"))
	if target == nil {
		target = map[string]any{}
	}
	if targetURL := strings.TrimSpace(stringValue(arguments, "target_url", "")); targetURL != "" {
		target["url"] = targetURL
	}
	if titleContains := strings.TrimSpace(stringValue(arguments, "title_contains", "")); titleContains != "" {
		target["title_contains"] = titleContains
	}
	if pageIndex, ok := browserAttachPageIndex(arguments["page_index"]); ok {
		target["page_index"] = pageIndex
	}
	if len(target) > 0 {
		merged["target"] = target
	}
	return merged
}

func buildBrowserAttachInput(browserKind string, snapshot taskcontext.TaskContextSnapshot, arguments map[string]any, useSnapshotTarget, allowEmptyTarget, requireStableTarget bool) map[string]any {
	target := map[string]any{}
	if pageIndex, ok := browserAttachPageIndex(arguments["page_index"]); ok {
		target["page_index"] = pageIndex
	}
	if targetURL := strings.TrimSpace(stringValue(arguments, "target_url", "")); targetURL != "" {
		target["url"] = targetURL
	} else if useSnapshotTarget && strings.TrimSpace(snapshot.PageURL) != "" {
		target["url"] = strings.TrimSpace(snapshot.PageURL)
	}
	if !requireStableTarget {
		if titleContains := strings.TrimSpace(stringValue(arguments, "title_contains", "")); titleContains != "" {
			target["title_contains"] = titleContains
		} else if useSnapshotTarget {
			if pageTitle := strings.TrimSpace(snapshot.PageTitle); pageTitle != "" {
				target["title_contains"] = pageTitle
			} else if windowTitle := strings.TrimSpace(snapshot.WindowTitle); windowTitle != "" {
				target["title_contains"] = windowTitle
			}
		}
	}
	if requireStableTarget && !hasStableBrowserTarget(target) {
		return nil
	}
	if len(target) == 0 && !allowEmptyTarget {
		return nil
	}

	attach := map[string]any{
		"mode":         string(tools.BrowserAttachModeCDP),
		"browser_kind": browserKind,
	}
	if len(target) > 0 {
		attach["target"] = target
	}
	return attach
}

func browserIntentTargetObject(intentName string, arguments map[string]any) string {
	if requiresStableBrowserTarget(intentName) {
		if strings.TrimSpace(intentName) == "browser_navigate" {
			if value := strings.TrimSpace(stringValue(arguments, "url", "")); value != "" {
				return value
			}
		}
		if targetURL := strings.TrimSpace(stringValue(arguments, "target_url", "")); targetURL != "" {
			return targetURL
		}
		if pageIndex, ok := browserAttachPageIndex(arguments["page_index"]); ok {
			return fmt.Sprintf("browser_tab:%d", pageIndex)
		}
		return browserStableTargetObject(mapValue(arguments, "attach"))
	}

	if strings.TrimSpace(intentName) == "browser_navigate" {
		if value := strings.TrimSpace(stringValue(arguments, "url", "")); value != "" {
			return value
		}
	}
	if targetURL := strings.TrimSpace(stringValue(arguments, "target_url", "")); targetURL != "" {
		return targetURL
	}
	if titleContains := strings.TrimSpace(stringValue(arguments, "title_contains", "")); titleContains != "" {
		return titleContains
	}
	if pageIndex, ok := browserAttachPageIndex(arguments["page_index"]); ok {
		return fmt.Sprintf("browser_tab:%d", pageIndex)
	}
	return browserTargetObject(mapValue(arguments, "attach"))
}

func browserStableTargetObject(attach map[string]any) string {
	if len(attach) == 0 {
		return ""
	}
	target := mapValue(attach, "target")
	if value := strings.TrimSpace(stringValue(target, "url", "")); value != "" {
		return value
	}
	if pageIndex, ok := browserAttachPageIndex(target["page_index"]); ok {
		return fmt.Sprintf("browser_tab:%d", pageIndex)
	}
	return ""
}

func browserTargetObject(attach map[string]any) string {
	if len(attach) == 0 {
		return ""
	}
	target := mapValue(attach, "target")
	if value := strings.TrimSpace(stringValue(target, "url", "")); value != "" {
		return value
	}
	if pageIndex, ok := browserAttachPageIndex(target["page_index"]); ok {
		return fmt.Sprintf("browser_tab:%d", pageIndex)
	}
	if value := strings.TrimSpace(stringValue(target, "title_contains", "")); value != "" {
		return value
	}
	return strings.TrimSpace(stringValue(attach, "browser_kind", ""))
}

func requiresStableBrowserTarget(intentName string) bool {
	switch strings.TrimSpace(intentName) {
	case "browser_navigate", "browser_tab_focus", "browser_interact":
		return true
	default:
		return false
	}
}

func hasStableBrowserAttachTarget(attach map[string]any) bool {
	return hasStableBrowserTarget(mapValue(attach, "target"))
}

func hasStableBrowserTarget(target map[string]any) bool {
	if len(target) == 0 {
		return false
	}
	if strings.TrimSpace(stringValue(target, "url", "")) != "" {
		return true
	}
	_, ok := browserAttachPageIndex(target["page_index"])
	return ok
}

func browserAttachPageIndex(rawValue any) (int, bool) {
	switch typed := rawValue.(type) {
	case int:
		if typed >= 0 {
			return typed, true
		}
	case float64:
		if typed >= 0 && typed == float64(int(typed)) {
			return int(typed), true
		}
	}
	return 0, false
}

func browserTargetOverrideMissing(arguments map[string]any) bool {
	if _, ok := browserAttachPageIndex(arguments["page_index"]); ok {
		return false
	}
	if strings.TrimSpace(stringValue(arguments, "target_url", "")) != "" {
		return false
	}
	return strings.TrimSpace(stringValue(arguments, "title_contains", "")) == ""
}

func resolvePageToolInput(intentName string, arguments map[string]any, snapshot taskcontext.TaskContextSnapshot) (map[string]any, bool) {
	urlValue := strings.TrimSpace(stringValue(arguments, "url", ""))
	if urlValue == "" {
		return nil, false
	}
	input := map[string]any{"url": urlValue}
	switch intentName {
	case "page_search":
		queryValue := strings.TrimSpace(stringValue(arguments, "query", ""))
		if queryValue == "" {
			return nil, false
		}
		input["query"] = queryValue
		if limit, ok := arguments["limit"]; ok {
			input["limit"] = limit
		}
	case "page_interact":
		if actions, ok := arguments["actions"]; ok {
			input["actions"] = actions
		}
	}
	if attach := pageAttachInput(urlValue, arguments, snapshot); len(attach) > 0 {
		input["attach"] = attach
	}
	return input, true
}

func resolveWebSearchToolInput(arguments map[string]any) (map[string]any, bool) {
	queryValue := strings.TrimSpace(stringValue(arguments, "query", ""))
	if queryValue == "" {
		return nil, false
	}

	// Keep the derived search URL for governance/audit targeting, but mark it as
	// implicit so the worker still treats "no parseable results from the default
	// search page" as a structured failure instead of a silent empty success.
	input := map[string]any{
		"query":           queryValue,
		"url":             defaultWebSearchURL(queryValue),
		"url_is_explicit": false,
	}
	if limit, ok := arguments["limit"]; ok {
		input["limit"] = limit
	}
	return input, true
}

func defaultWebSearchURL(query string) string {
	if strings.TrimSpace(query) == "" {
		return ""
	}
	return "https://duckduckgo.com/html/?q=" + url.QueryEscape(strings.TrimSpace(query))
}

func pageAttachInput(urlValue string, arguments map[string]any, snapshot taskcontext.TaskContextSnapshot) map[string]any {
	// Page-level attach hints must come from trusted desktop context. The planner
	// can request a page tool, but it must not steer browser kind or CDP endpoint
	// away from the observed foreground session.
	browserKind := strings.ToLower(strings.TrimSpace(snapshot.BrowserKind))
	if browserKind != "chrome" && browserKind != "edge" {
		return nil
	}
	pageURL := comparablePageURL(snapshot.PageURL)
	requestURL := comparablePageURL(urlValue)
	if pageURL == "" || requestURL == "" || pageURL != requestURL {
		return nil
	}
	target := map[string]any{"url": pageURL}
	if pageTitle := strings.TrimSpace(snapshot.PageTitle); pageTitle != "" {
		target["title_contains"] = pageTitle
	}
	attach := map[string]any{
		"mode":         string(tools.BrowserAttachModeCDP),
		"browser_kind": browserKind,
		"target":       target,
	}
	return attach
}

func comparablePageURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	parsed.Scheme = strings.ToLower(strings.TrimSpace(parsed.Scheme))
	hostname := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	port := strings.TrimSpace(parsed.Port())
	switch {
	case hostname == "":
		parsed.Host = ""
	case port == "":
		parsed.Host = hostname
	case parsed.Scheme == "http" && port == "80":
		parsed.Host = hostname
	case parsed.Scheme == "https" && port == "443":
		parsed.Host = hostname
	default:
		parsed.Host = net.JoinHostPort(hostname, port)
	}
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	parsed.Fragment = ""
	return parsed.String()
}

func requireAuthorizationFlag(intent map[string]any) bool {
	return boolValue(mapValue(intent, "arguments"), "require_authorization")
}

func resolveWorkspaceRoot(fileSystem platform.FileSystemAdapter) string {
	if fileSystem == nil {
		return ""
	}

	workspaceRoot, err := fileSystem.EnsureWithinWorkspace(".")
	if err != nil {
		return ""
	}
	return workspaceRoot
}

func latestToolCall(toolCalls []tools.ToolCallRecord) tools.ToolCallRecord {
	if len(toolCalls) == 0 {
		return tools.ToolCallRecord{}
	}
	return toolCalls[len(toolCalls)-1]
}

func internalScreenAnalysisCapabilities(request Request) []string {
	capabilities := []string{"ocr_image"}
	arguments := mapValue(request.Intent, "arguments")
	if tools.ScreenCaptureMode(stringValue(arguments, "capture_mode", string(tools.ScreenCaptureModeScreenshot))) == tools.ScreenCaptureModeClip {
		capabilities = append(capabilities, "extract_frames")
	}
	return capabilities
}
