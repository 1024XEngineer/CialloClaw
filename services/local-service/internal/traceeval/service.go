package traceeval

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/textutil"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

// Service records execution trace/eval data and detects doom-loop / human review
// escalation signals for owner-5 governance flows.
type Service struct {
	traceStore storage.TraceStore
	evalStore  storage.EvalStore
	now        func() time.Time
}

// CaptureInput describes the minimal execution context needed to build trace
// and eval records.
type CaptureInput struct {
	TaskID          string
	RunID           string
	IntentName      string
	Snapshot        taskcontext.TaskContextSnapshot
	OutputText      string
	DeliveryResult  map[string]any
	Artifacts       []map[string]any
	ExtensionAssets []storage.ExtensionAssetReference
	ModelInvocation map[string]any
	ToolCalls       []tools.ToolCallRecord
	TokenUsage      map[string]any
	DurationMS      int64
	ExecutionError  error
}

// DoomLoopResult describes the detected doom-loop condition.
type DoomLoopResult struct {
	Triggered   bool
	Reason      string
	RepeatCount int
	Trigger     string
}

// HumanLoopEscalation describes one structured human-in-the-loop escalation.
type HumanLoopEscalation struct {
	EscalationID    string
	TaskID          string
	RunID           string
	Reason          string
	ReviewResult    string
	Status          string
	Summary         string
	SuggestedAction string
	CreatedAt       string
}

// CaptureResult bundles the generated trace/eval records with loop/escalation
// decisions.
type CaptureResult struct {
	TraceRecord  storage.TraceRecord
	EvalSnapshot storage.EvalSnapshotRecord
	DoomLoop     DoomLoopResult
	HumanInLoop  *HumanLoopEscalation
	RuleHits     []string
	ReviewResult string
	EvalStatus   string
	Metrics      map[string]any
}

// NewService creates a trace/eval service.
func NewService(traceStore storage.TraceStore, evalStore storage.EvalStore) *Service {
	return &Service{traceStore: traceStore, evalStore: evalStore, now: time.Now}
}

// Capture builds trace/eval records and detects doom-loop / human escalation.
func (s *Service) Capture(input CaptureInput) (CaptureResult, error) {
	now := s.now().UTC()
	doomLoop := detectDoomLoop(input.ToolCalls)
	reviewResult := "passed"
	if doomLoop.Triggered {
		reviewResult = "human_review_required"
	} else if input.ExecutionError != nil || countFailedToolCalls(input.ToolCalls) > 0 {
		reviewResult = "needs_attention"
	}
	evalStatus := reviewResult
	if evalStatus == "passed" {
		evalStatus = "passed"
	}
	ruleHits := buildRuleHits(input, doomLoop, reviewResult)
	normalizedAssets := storage.NormalizeExtensionAssetReferences(input.ExtensionAssets)
	ruleHitsJSON, err := json.Marshal(ruleHits)
	if err != nil {
		return CaptureResult{}, fmt.Errorf("marshal trace rule hits: %w", err)
	}
	assetRefsJSON, err := json.Marshal(normalizedAssets)
	if err != nil {
		return CaptureResult{}, fmt.Errorf("marshal extension asset refs: %w", err)
	}
	traceID := fmt.Sprintf("trace_%d", now.UnixNano())
	metrics := map[string]any{
		"latency_ms":            resolveLatency(input),
		"tool_call_count":       len(input.ToolCalls),
		"failed_tool_calls":     countFailedToolCalls(input.ToolCalls),
		"artifact_count":        len(input.Artifacts),
		"loop_round":            maxLoopRound(input.ToolCalls),
		"doom_loop_triggered":   doomLoop.Triggered,
		"review_result":         reviewResult,
		"human_in_loop":         doomLoop.Triggered,
		"delivery_type":         stringValue(input.DeliveryResult, "type"),
		"error_present":         input.ExecutionError != nil,
		"extension_asset_count": len(normalizedAssets),
	}
	mergeExtensionAssetMetrics(metrics, normalizedAssets)
	mergeTokenMetrics(metrics, input.TokenUsage, input.ModelInvocation)
	metricsJSON, err := json.Marshal(metrics)
	if err != nil {
		return CaptureResult{}, fmt.Errorf("marshal eval metrics: %w", err)
	}
	traceRecord := storage.TraceRecord{
		TraceID:          traceID,
		TaskID:           input.TaskID,
		RunID:            strings.TrimSpace(input.RunID),
		LoopRound:        maxLoopRound(input.ToolCalls),
		LLMInputSummary:  buildInputSummary(input),
		LLMOutputSummary: buildOutputSummary(input),
		LatencyMS:        resolveLatency(input),
		Cost:             resolveCost(input.TokenUsage),
		AssetRefsJSON:    string(assetRefsJSON),
		RuleHitsJSON:     string(ruleHitsJSON),
		ReviewResult:     reviewResult,
		CreatedAt:        now.Format(time.RFC3339),
	}
	evalRecord := storage.EvalSnapshotRecord{
		EvalSnapshotID: fmt.Sprintf("eval_%d", now.UnixNano()),
		TraceID:        traceID,
		TaskID:         input.TaskID,
		Status:         evalStatus,
		AssetRefsJSON:  string(assetRefsJSON),
		MetricsJSON:    string(metricsJSON),
		CreatedAt:      now.Format(time.RFC3339),
	}
	result := CaptureResult{
		TraceRecord:  traceRecord,
		EvalSnapshot: evalRecord,
		DoomLoop:     doomLoop,
		RuleHits:     ruleHits,
		ReviewResult: reviewResult,
		EvalStatus:   evalStatus,
		Metrics:      metrics,
	}
	if doomLoop.Triggered {
		result.HumanInLoop = &HumanLoopEscalation{
			EscalationID:    fmt.Sprintf("hitl_%d", now.UnixNano()),
			TaskID:          input.TaskID,
			RunID:           input.RunID,
			Reason:          doomLoop.Reason,
			ReviewResult:    reviewResult,
			Status:          "pending",
			Summary:         fmt.Sprintf("Potential Doom Loop detected: %s. Human review is required.", doomLoop.Reason),
			SuggestedAction: "review_and_replan",
			CreatedAt:       now.Format(time.RFC3339),
		}
	}
	return result, nil
}

// Record persists the generated trace/eval records when backing stores exist.
func (s *Service) Record(ctx context.Context, result CaptureResult) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if s.traceStore != nil {
		if err := s.traceStore.WriteTraceRecord(ctx, result.TraceRecord); err != nil {
			return err
		}
	}
	if s.evalStore != nil {
		if err := s.evalStore.WriteEvalSnapshot(ctx, result.EvalSnapshot); err != nil {
			if s.traceStore != nil {
				if rollbackErr := s.traceStore.DeleteTraceRecord(ctx, result.TraceRecord.TraceID); rollbackErr != nil {
					return fmt.Errorf("rollback trace record after eval write failure: %w", rollbackErr)
				}
			}
			return err
		}
	}
	return nil
}

func detectDoomLoop(toolCalls []tools.ToolCallRecord) DoomLoopResult {
	if len(toolCalls) < 3 {
		return DoomLoopResult{}
	}
	if repeated := repeatedCallSignature(toolCalls); repeated.count >= 3 {
		return DoomLoopResult{Triggered: true, Reason: repeated.reason, RepeatCount: repeated.count, Trigger: "repeated_call_signature"}
	}
	if repeatedFailure := repeatedNoProgressFailure(toolCalls); repeatedFailure.count >= 3 {
		return DoomLoopResult{Triggered: true, Reason: repeatedFailure.reason, RepeatCount: repeatedFailure.count, Trigger: "repeated_no_progress_failure"}
	}
	return DoomLoopResult{}
}

type repeatedErrorResult struct {
	count  int
	reason string
}

func repeatedCallSignature(toolCalls []tools.ToolCallRecord) repeatedErrorResult {
	best := repeatedErrorResult{}
	repeat := 1
	for index := 1; index < len(toolCalls); index++ {
		currentSignature := callSignature(toolCalls[index])
		previousSignature := callSignature(toolCalls[index-1])
		if currentSignature != "" && currentSignature == previousSignature {
			repeat++
			if repeat > best.count {
				best = repeatedErrorResult{
					count:  repeat,
					reason: fmt.Sprintf("call signature repeated %d times for tool %s", repeat, toolCalls[index].ToolName),
				}
			}
			continue
		}
		repeat = 1
	}
	return best
}

func maxLoopRound(toolCalls []tools.ToolCallRecord) int {
	maxRound := 0
	for _, toolCall := range toolCalls {
		loopRound := intValue(toolCall.Output, "loop_round")
		if loopRound > maxRound {
			maxRound = loopRound
		}
	}
	return maxRound
}

func repeatedNoProgressFailure(toolCalls []tools.ToolCallRecord) repeatedErrorResult {
	best := repeatedErrorResult{}
	repeat := 1
	for index := 1; index < len(toolCalls); index++ {
		current := toolCalls[index]
		previous := toolCalls[index-1]
		if !isNoProgressFailure(current) || !isNoProgressFailure(previous) {
			repeat = 1
			continue
		}
		currentSignature := failureSignature(current)
		previousSignature := failureSignature(previous)
		if currentSignature != "" && currentSignature == previousSignature {
			repeat++
			if repeat > best.count {
				errorCode := 0
				if current.ErrorCode != nil {
					errorCode = *current.ErrorCode
				}
				best = repeatedErrorResult{
					count:  repeat,
					reason: fmt.Sprintf("tool %s repeated no-progress failure with error code %d", current.ToolName, errorCode),
				}
			}
			continue
		}
		repeat = 1
	}
	return best
}

func buildRuleHits(input CaptureInput, doomLoop DoomLoopResult, reviewResult string) []string {
	ruleHits := make([]string, 0, 6)
	if len(input.ToolCalls) > 0 {
		ruleHits = append(ruleHits, fmt.Sprintf("tool_calls=%d", len(input.ToolCalls)))
	}
	if workerCalls := countWorkerCalls(input.ToolCalls); workerCalls > 0 {
		ruleHits = append(ruleHits, fmt.Sprintf("worker_calls=%d", workerCalls))
	}
	if maxRound := maxLoopRound(input.ToolCalls); maxRound > 0 {
		ruleHits = append(ruleHits, fmt.Sprintf("loop_round=%d", maxRound))
	}
	if doomLoop.Triggered {
		ruleHits = append(ruleHits, "doom_loop="+doomLoop.Trigger)
	}
	if reviewResult != "passed" {
		ruleHits = append(ruleHits, "review_result="+reviewResult)
	}
	if strings.TrimSpace(stringValue(input.DeliveryResult, "type")) != "" {
		ruleHits = append(ruleHits, "delivery_type="+stringValue(input.DeliveryResult, "type"))
	}
	sort.Strings(ruleHits)
	return ruleHits
}

func buildInputSummary(input CaptureInput) string {
	for _, textInput := range []struct {
		label string
		value string
	}{
		{label: "selection_text", value: input.Snapshot.SelectionText},
		{label: "task_text", value: input.Snapshot.Text},
		{label: "error_text", value: input.Snapshot.ErrorText},
	} {
		if strings.TrimSpace(textInput.value) != "" {
			return hashTextSummary(textInput.label, textInput.value)
		}
	}
	if len(input.Snapshot.Files) > 0 {
		return input.Snapshot.Files[0]
	}
	for _, textInput := range []struct {
		label string
		value string
	}{
		{label: "page_title", value: input.Snapshot.PageTitle},
		{label: "window_title", value: input.Snapshot.WindowTitle},
		{label: "visible_text", value: input.Snapshot.VisibleText},
		{label: "clipboard_text", value: input.Snapshot.ClipboardText},
	} {
		if strings.TrimSpace(textInput.value) != "" {
			return hashTextSummary(textInput.label, textInput.value)
		}
	}
	if strings.TrimSpace(input.IntentName) != "" {
		return input.IntentName
	}
	return "trace_input"
}

func buildOutputSummary(input CaptureInput) string {
	if strings.TrimSpace(input.OutputText) != "" {
		return hashTextSummary("output_text", input.OutputText)
	}
	if len(input.ToolCalls) > 0 {
		lastToolCall := input.ToolCalls[len(input.ToolCalls)-1]
		if strings.TrimSpace(lastToolCall.ToolName) != "" {
			return fmt.Sprintf("last tool: %s", lastToolCall.ToolName)
		}
	}
	if input.ExecutionError != nil {
		return hashTextSummary("execution_error", input.ExecutionError.Error())
	}
	return "trace_output"
}

func resolveLatency(input CaptureInput) int64 {
	if latency := int64Value(input.ModelInvocation, "latency_ms"); latency > 0 {
		return latency
	}
	if input.DurationMS > 0 {
		return input.DurationMS
	}
	return 0
}

func resolveCost(tokenUsage map[string]any) float64 {
	if value, ok := tokenUsage["estimated_cost"].(float64); ok {
		return value
	}
	if value, ok := tokenUsage["estimated_cost"].(int); ok {
		return float64(value)
	}
	return 0
}

func mergeTokenMetrics(metrics map[string]any, tokenUsage map[string]any, modelInvocation map[string]any) {
	usage := mapValue(modelInvocation, "usage")
	if len(tokenUsage) > 0 {
		for key, value := range tokenUsage {
			metrics[key] = value
		}
	}
	for _, key := range []string{"input_tokens", "output_tokens", "total_tokens"} {
		if _, ok := metrics[key]; ok {
			continue
		}
		if value, ok := usage[key]; ok {
			metrics[key] = value
		}
	}
}

func mergeExtensionAssetMetrics(metrics map[string]any, refs []storage.ExtensionAssetReference) {
	if len(refs) == 0 {
		return
	}
	counts := map[string]int{}
	for _, ref := range refs {
		counts[ref.AssetKind]++
	}
	for kind, count := range counts {
		metrics[kind+"_count"] = count
	}
}

func countFailedToolCalls(toolCalls []tools.ToolCallRecord) int {
	count := 0
	for _, toolCall := range toolCalls {
		if toolCall.Status == tools.ToolCallStatusFailed || toolCall.Status == tools.ToolCallStatusTimeout {
			count++
		}
	}
	return count
}

func countWorkerCalls(toolCalls []tools.ToolCallRecord) int {
	count := 0
	for _, toolCall := range toolCalls {
		if source := stringValue(toolCall.Output, "source"); strings.HasSuffix(source, "_worker") || strings.HasSuffix(source, "_sidecar") {
			count++
		}
	}
	return count
}

func callSignature(toolCall tools.ToolCallRecord) string {
	toolName := strings.TrimSpace(toolCall.ToolName)
	if toolName == "" {
		return ""
	}
	inputJSON, err := json.Marshal(toolCall.Input)
	if err != nil {
		return toolName
	}
	return toolName + ":" + string(inputJSON)
}

func failureSignature(toolCall tools.ToolCallRecord) string {
	if !isNoProgressFailure(toolCall) {
		return ""
	}
	errorCode := 0
	if toolCall.ErrorCode != nil {
		errorCode = *toolCall.ErrorCode
	}
	return fmt.Sprintf("%s:%d:%s", strings.TrimSpace(toolCall.ToolName), errorCode, callSignature(toolCall))
}

func isNoProgressFailure(toolCall tools.ToolCallRecord) bool {
	return toolCall.Status == tools.ToolCallStatusFailed || toolCall.Status == tools.ToolCallStatusTimeout || toolCall.ErrorCode != nil
}

func hashTextSummary(label string, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(trimmed))
	return fmt.Sprintf("%s#%x", label, hash[:6])
}

func mapValue(values map[string]any, key string) map[string]any {
	rawValue, ok := values[key]
	if !ok {
		return map[string]any{}
	}
	value, ok := rawValue.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return value
}

func stringValue(values map[string]any, key string) string {
	rawValue, ok := values[key]
	if !ok {
		return ""
	}
	value, ok := rawValue.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func intValue(values map[string]any, key string) int {
	rawValue, ok := values[key]
	if !ok {
		return 0
	}
	switch typed := rawValue.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func int64Value(values map[string]any, key string) int64 {
	rawValue, ok := values[key]
	if !ok {
		return 0
	}
	switch typed := rawValue.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	default:
		return 0
	}
}

func truncateText(value string, maxLength int) string {
	value = strings.TrimSpace(value)
	return textutil.TruncateGraphemes(value, maxLength)
}
