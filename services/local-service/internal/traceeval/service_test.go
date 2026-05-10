package traceeval

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func TestServiceCaptureBuildsTraceAndEvalRecords(t *testing.T) {
	service := NewService(storage.NewService(nil).TraceStore(), storage.NewService(nil).EvalStore())
	service.now = func() time.Time { return time.Date(2026, 4, 17, 9, 0, 0, 0, time.UTC) }
	result, err := service.Capture(CaptureInput{
		TaskID:     "task_trace",
		RunID:      "run_trace",
		IntentName: "agent_loop",
		Snapshot: taskcontext.TaskContextSnapshot{
			Text:      "please inspect this note",
			PageTitle: "Release Dashboard",
		},
		OutputText: "Here is the final answer.",
		DeliveryResult: map[string]any{
			"type": "workspace_document",
		},
		Artifacts: []map[string]any{{"artifact_id": "art_001"}},
		ExtensionAssets: []storage.ExtensionAssetReference{{
			AssetKind: storage.ExtensionAssetKindSkillManifest,
			AssetID:   "skill_builtin_default_agent_loop",
			Name:      "default_agent_loop_skill",
			Version:   "builtin-v1",
			Source:    "builtin",
		}},
		ModelInvocation: map[string]any{
			"latency_ms": int64(321),
			"usage": map[string]any{
				"input_tokens":  20,
				"output_tokens": 40,
				"total_tokens":  60,
			},
		},
		ToolCalls: []tools.ToolCallRecord{{
			ToolName: "read_file",
			Status:   tools.ToolCallStatusSucceeded,
			Output:   map[string]any{"loop_round": 1, "source": "ocr_worker"},
		}},
		TokenUsage: map[string]any{"estimated_cost": 0.012},
		DurationMS: 900,
	})
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	if result.TraceRecord.TraceID == "" || result.EvalSnapshot.EvalSnapshotID == "" {
		t.Fatalf("expected trace/eval ids, got %+v", result)
	}
	if result.TraceRecord.ReviewResult != "passed" || result.EvalSnapshot.Status != "passed" {
		t.Fatalf("expected passing review/eval status, got %+v", result)
	}
	if result.TraceRecord.LoopRound != 1 || result.TraceRecord.Cost != 0.012 {
		t.Fatalf("expected loop/cost metrics to be captured, got %+v", result.TraceRecord)
	}
	if result.TraceRecord.AssetRefsJSON == "" || result.EvalSnapshot.AssetRefsJSON == "" {
		t.Fatalf("expected asset refs to be persisted into trace/eval records, got %+v", result)
	}
	if result.Metrics["skill_manifest_count"] != 1 {
		t.Fatalf("expected extension asset metrics to count skill manifest usage, got %+v", result.Metrics)
	}
	if result.Metrics["worker_calls"] != nil {
		// worker_calls is stored in rule hits, not metrics
	}
	if err := service.Record(context.Background(), result); err != nil {
		t.Fatalf("record failed: %v", err)
	}
	items, total, err := service.traceStore.ListTraceRecords(context.Background(), "task_trace", 10, 0)
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("expected one persisted trace record, total=%d len=%d err=%v", total, len(items), err)
	}
	evals, total, err := service.evalStore.ListEvalSnapshots(context.Background(), "task_trace", 10, 0)
	if err != nil || total != 1 || len(evals) != 1 {
		t.Fatalf("expected one persisted eval snapshot, total=%d len=%d err=%v", total, len(evals), err)
	}
}

func TestServiceCaptureEscalatesDoomLoopToHumanReview(t *testing.T) {
	service := NewService(nil, nil)
	service.now = func() time.Time { return time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC) }
	result, err := service.Capture(CaptureInput{
		TaskID:     "task_loop",
		RunID:      "run_loop",
		IntentName: "agent_loop",
		Snapshot:   taskcontext.TaskContextSnapshot{Text: "keep trying"},
		ToolCalls: []tools.ToolCallRecord{
			{ToolName: "read_file", Input: map[string]any{"path": "workspace/a.md"}, Status: tools.ToolCallStatusFailed, ErrorCode: intPtr(1001), Output: map[string]any{"loop_round": 3}},
			{ToolName: "read_file", Input: map[string]any{"path": "workspace/a.md"}, Status: tools.ToolCallStatusFailed, ErrorCode: intPtr(1001), Output: map[string]any{"loop_round": 3}},
			{ToolName: "read_file", Input: map[string]any{"path": "workspace/a.md"}, Status: tools.ToolCallStatusFailed, ErrorCode: intPtr(1001), Output: map[string]any{"loop_round": 3}},
		},
		DurationMS: 600,
	})
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	if !result.DoomLoop.Triggered || result.HumanInLoop == nil {
		t.Fatalf("expected doom-loop escalation, got %+v", result)
	}
	if result.ReviewResult != "human_review_required" || result.EvalStatus != "human_review_required" {
		t.Fatalf("expected human review statuses, got %+v", result)
	}
	if result.HumanInLoop.Status != "pending" || result.HumanInLoop.TaskID != "task_loop" {
		t.Fatalf("expected structured human escalation payload, got %+v", result.HumanInLoop)
	}
}

func TestDetectDoomLoopIgnoresRepeatedProgressingToolUsage(t *testing.T) {
	doomLoop := detectDoomLoop([]tools.ToolCallRecord{
		{ToolName: "read_file", Input: map[string]any{"path": "workspace/a.md"}, Status: tools.ToolCallStatusSucceeded, Output: map[string]any{"loop_round": 1}},
		{ToolName: "read_file", Input: map[string]any{"path": "workspace/b.md"}, Status: tools.ToolCallStatusSucceeded, Output: map[string]any{"loop_round": 2}},
		{ToolName: "read_file", Input: map[string]any{"path": "workspace/c.md"}, Status: tools.ToolCallStatusSucceeded, Output: map[string]any{"loop_round": 3}},
	})
	if doomLoop.Triggered {
		t.Fatalf("expected progressing repeated tool usage to avoid doom-loop escalation, got %+v", doomLoop)
	}
}

func TestTraceEvalHelpersCoverErrorAndFilePriorityBranches(t *testing.T) {
	toolCalls := []tools.ToolCallRecord{
		{ToolName: "read_file", Input: map[string]any{"path": "workspace/specs/report.md"}, Status: tools.ToolCallStatusFailed, ErrorCode: intPtr(1001)},
		{ToolName: "read_file", Input: map[string]any{"path": "workspace/specs/report.md"}, Status: tools.ToolCallStatusFailed, ErrorCode: intPtr(1001)},
		{ToolName: "read_file", Input: map[string]any{"path": "workspace/specs/report.md"}, Status: tools.ToolCallStatusFailed, ErrorCode: intPtr(1001)},
		{ToolName: "page_read", Status: tools.ToolCallStatusSucceeded},
	}
	doomLoop := detectDoomLoop(toolCalls)
	if !doomLoop.Triggered || doomLoop.Trigger != "repeated_call_signature" {
		t.Fatalf("expected repeated tool errors to trigger doom loop, got %+v", doomLoop)
	}
	input := CaptureInput{
		TaskID:     "task_file",
		RunID:      "run_file",
		IntentName: "agent_loop",
		Snapshot: taskcontext.TaskContextSnapshot{
			Files:         []string{"workspace/specs/report.md"},
			VisibleText:   "visible page text",
			ClipboardText: "clipboard",
		},
		ToolCalls:      toolCalls,
		DurationMS:     500,
		ExecutionError: errors.New("execution failed"),
		ModelInvocation: map[string]any{
			"usage": map[string]any{"input_tokens": 12, "output_tokens": 4, "total_tokens": 16},
		},
	}
	if buildInputSummary(input) != "workspace/specs/report.md" {
		t.Fatalf("expected file input to outrank perception text, got %q", buildInputSummary(input))
	}
	textInput := CaptureInput{Snapshot: taskcontext.TaskContextSnapshot{SelectionText: "secret copied token", ClipboardText: "another secret"}}
	textSummary := buildInputSummary(textInput)
	if strings.Contains(textSummary, "secret copied token") || strings.Contains(textSummary, "another secret") {
		t.Fatalf("expected hashed trace input summary instead of raw text, got %q", textSummary)
	}
	if buildOutputSummary(input) != "last tool: page_read" {
		t.Fatalf("expected last tool summary fallback, got %q", buildOutputSummary(input))
	}
	outputSummary := buildOutputSummary(CaptureInput{OutputText: "secret generated output"})
	if strings.Contains(outputSummary, "secret generated output") {
		t.Fatalf("expected hashed output summary instead of raw output text, got %q", outputSummary)
	}
	errorSummary := buildOutputSummary(CaptureInput{ExecutionError: errors.New("sensitive failure payload")})
	if strings.Contains(errorSummary, "sensitive failure payload") {
		t.Fatalf("expected hashed output summary instead of raw error text, got %q", errorSummary)
	}
	metrics := map[string]any{}
	mergeTokenMetrics(metrics, nil, input.ModelInvocation)
	if metrics["total_tokens"] != 16 {
		t.Fatalf("expected mergeTokenMetrics to copy invocation usage, got %+v", metrics)
	}
	if countFailedToolCalls([]tools.ToolCallRecord{{Status: tools.ToolCallStatusTimeout}}) != 1 {
		t.Fatal("expected timeout to count as failed tool call")
	}
	if countWorkerCalls([]tools.ToolCallRecord{{Output: map[string]any{"source": "ocr_worker"}}}) != 1 {
		t.Fatal("expected worker source to count as worker call")
	}
	service := NewService(nil, nil)
	service.now = func() time.Time { return time.Date(2026, 4, 17, 11, 0, 0, 0, time.UTC) }
	result, err := service.Capture(input)
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	if result.ReviewResult != "human_review_required" || result.Metrics["error_present"] != true {
		t.Fatalf("expected error path to mark review attention, got %+v", result)
	}
	if err := service.Record(context.Background(), result); err != nil {
		t.Fatalf("record with nil stores should be no-op, got %v", err)
	}
}

func TestServiceRecordReturnsEvalWriteErrorAfterTraceWrite(t *testing.T) {
	traceStore := storage.NewService(nil).TraceStore()
	service := NewService(traceStore, failingEvalStore{err: fmt.Errorf("eval write failed")})
	service.now = func() time.Time { return time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC) }

	result, err := service.Capture(CaptureInput{TaskID: "task_trace_eval_error", RunID: "run_trace_eval_error", IntentName: "summarize"})
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	err = service.Record(context.Background(), result)
	if err == nil || !strings.Contains(err.Error(), "eval write failed") {
		t.Fatalf("expected eval write failure to surface, got %v", err)
	}
	items, total, err := traceStore.ListTraceRecords(context.Background(), "task_trace_eval_error", 10, 0)
	if err != nil || total != 0 || len(items) != 0 {
		t.Fatalf("expected trace record rollback after eval failure, total=%d len=%d err=%v", total, len(items), err)
	}
}

func TestServiceRecordReturnsRollbackErrorWhenTraceCleanupFails(t *testing.T) {
	service := NewService(failingTraceStore{deleteErr: fmt.Errorf("trace rollback failed")}, failingEvalStore{err: fmt.Errorf("eval write failed")})
	service.now = func() time.Time { return time.Date(2026, 4, 17, 12, 30, 0, 0, time.UTC) }

	result, err := service.Capture(CaptureInput{TaskID: "task_trace_rollback_error", RunID: "run_trace_rollback_error", IntentName: "summarize"})
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	err = service.Record(context.Background(), result)
	if err == nil || !strings.Contains(err.Error(), "trace rollback failed") {
		t.Fatalf("expected rollback failure to surface, got %v", err)
	}
}

func TestTraceEvalHelperBranches(t *testing.T) {
	toolCall := tools.ToolCallRecord{ToolName: "read_file", Status: tools.ToolCallStatusFailed, ErrorCode: intPtr(1001), Input: map[string]any{"path": "workspace/a.md"}}
	if signature := failureSignature(toolCall); !strings.Contains(signature, "read_file:1001:") {
		t.Fatalf("expected failureSignature to include tool and error code, got %q", signature)
	}
	if failureSignature(tools.ToolCallRecord{ToolName: "read_file", Status: tools.ToolCallStatusSucceeded}) != "" {
		t.Fatal("expected failureSignature to ignore successful tool calls")
	}
	if !isNoProgressFailure(tools.ToolCallRecord{Status: tools.ToolCallStatusTimeout}) {
		t.Fatal("expected timeout to count as no-progress failure")
	}
	if got := truncateText("  hello world  ", 5); got != "he..." {
		t.Fatalf("unexpected truncateText output: %q", got)
	}
	if got := truncateText("  hi  ", 5); got != "hi" {
		t.Fatalf("expected short strings to stay intact, got %q", got)
	}
	if intValue(map[string]any{"count": int64(3)}, "count") != 3 || intValue(map[string]any{"count": 4.0}, "count") != 4 || intValue(map[string]any{"count": "bad"}, "count") != 0 {
		t.Fatal("expected intValue to handle int64, float64, and invalid inputs")
	}
	if int64Value(map[string]any{"count": int(3)}, "count") != 3 || int64Value(map[string]any{"count": 4.0}, "count") != 4 || int64Value(map[string]any{"count": "bad"}, "count") != 0 {
		t.Fatal("expected int64Value to handle int, float64, and invalid inputs")
	}
	if callSignature(tools.ToolCallRecord{ToolName: "read_file", Input: map[string]any{"bad": func() {}}}) != "read_file" {
		t.Fatal("expected callSignature to fall back to tool name when input cannot be marshaled")
	}
	if repeatedNoProgressFailure([]tools.ToolCallRecord{{ToolName: "read_file", Status: tools.ToolCallStatusFailed, ErrorCode: intPtr(1001), Input: map[string]any{"path": "a"}}, {ToolName: "read_file", Status: tools.ToolCallStatusFailed, ErrorCode: intPtr(1002), Input: map[string]any{"path": "a"}}}).count != 0 {
		t.Fatal("expected repeatedNoProgressFailure to ignore mismatched failure signatures")
	}
}

func TestTraceEvalAdditionalHelperBranches(t *testing.T) {
	metrics := map[string]any{}
	mergeExtensionAssetMetrics(metrics, []storage.ExtensionAssetReference{
		{AssetKind: storage.ExtensionAssetKindSkillManifest},
		{AssetKind: storage.ExtensionAssetKindSkillManifest},
		{AssetKind: storage.ExtensionAssetKindPluginManifest},
	})
	if metrics[storage.ExtensionAssetKindSkillManifest+"_count"] != 2 || metrics[storage.ExtensionAssetKindPluginManifest+"_count"] != 1 {
		t.Fatalf("expected extension asset metrics to count asset kinds, got %+v", metrics)
	}
	if resolveCost(map[string]any{"estimated_cost": 2}) != 2 || resolveCost(map[string]any{}) != 0 {
		t.Fatal("expected resolveCost to handle integer and missing values")
	}
	if resolveLatency(CaptureInput{ModelInvocation: map[string]any{"latency_ms": int64(25)}}) != 25 {
		t.Fatal("expected resolveLatency to prefer model invocation latency")
	}
	if resolveLatency(CaptureInput{DurationMS: 17}) != 17 {
		t.Fatal("expected resolveLatency to fall back to capture duration")
	}
	if maxLoopRound([]tools.ToolCallRecord{{Output: map[string]any{"loop_round": int64(1)}}, {Output: map[string]any{"loop_round": float64(3)}}}) != 3 {
		t.Fatal("expected maxLoopRound to read int64 and float64 loop rounds")
	}
	if repeatedCallSignature([]tools.ToolCallRecord{{ToolName: "read_file", Input: map[string]any{"path": "workspace/a.md"}}, {ToolName: "read_file", Input: map[string]any{"path": "workspace/a.md"}}}).count != 2 {
		t.Fatal("expected repeatedCallSignature to count matching adjacent tool calls")
	}
	if repeatedCallSignature([]tools.ToolCallRecord{{ToolName: "read_file", Input: map[string]any{"bad": func() {}}}, {ToolName: "read_file", Input: map[string]any{"bad": func() {}}}}).count != 2 {
		t.Fatal("expected repeatedCallSignature to fall back to tool name when marshal fails")
	}
	if countWorkerCalls([]tools.ToolCallRecord{{Output: map[string]any{"source": "playwright_sidecar"}}}) != 1 {
		t.Fatal("expected sidecar source to count as worker-style call")
	}
	if mapValue(nil, "missing") == nil || stringValue(nil, "missing") != "" || intValue(map[string]any{"count": int64(3)}, "count") != 3 || int64Value(map[string]any{"count": int(3)}, "count") != 3 {
		t.Fatal("expected primitive helpers to tolerate nil maps and mixed numeric types")
	}
	if truncateText("  hello world  ", 5) != "he..." || truncateText(" hi ", 5) != "hi" {
		t.Fatal("expected truncateText to trim and truncate appropriately")
	}
}

type failingEvalStore struct {
	err error
}

func (s failingEvalStore) WriteEvalSnapshot(context.Context, storage.EvalSnapshotRecord) error {
	return s.err
}

func (s failingEvalStore) ListEvalSnapshots(context.Context, string, int, int) ([]storage.EvalSnapshotRecord, int, error) {
	return nil, 0, s.err
}

type failingTraceStore struct {
	writeErr  error
	deleteErr error
}

func (s failingTraceStore) WriteTraceRecord(context.Context, storage.TraceRecord) error {
	return s.writeErr
}

func (s failingTraceStore) DeleteTraceRecord(context.Context, string) error {
	return s.deleteErr
}

func (s failingTraceStore) ListTraceRecords(context.Context, string, int, int) ([]storage.TraceRecord, int, error) {
	return nil, 0, nil
}

func intPtr(value int) *int {
	return &value
}
