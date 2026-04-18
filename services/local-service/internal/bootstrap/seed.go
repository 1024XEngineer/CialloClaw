package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

const seedMarkerRelativePath = "data/seed-state.json"

type bootstrapSeedManifest struct {
	SeedVersion    string   `json:"seed_version"`
	SeedName       string   `json:"seed_name"`
	Description    string   `json:"description"`
	TodoStateFile  string   `json:"todo_state_file"`
	TasksFile      string   `json:"tasks_file"`
	MemoryFile     string   `json:"memory_file"`
	GovernanceFile string   `json:"governance_file"`
	SettingsFile   string   `json:"settings_file"`
	WorkspaceDir   string   `json:"workspace_dir"`
	Files          []string `json:"files"`
}

type bootstrapSeedBundle struct {
	Manifest   bootstrapSeedManifest
	TodoState  bootstrapSeedTodoState
	Tasks      bootstrapSeedTaskState
	Memory     bootstrapSeedMemoryState
	Governance bootstrapSeedGovernanceState
	Settings   bootstrapSeedSettingsState
}

type bootstrapSeedTodoState struct {
	Items []bootstrapSeedTodoItem `json:"items"`
	Rules []bootstrapSeedTodoRule `json:"rules"`
}

type bootstrapSeedTodoItem struct {
	ItemID           string           `json:"item_id"`
	Title            string           `json:"title"`
	Bucket           string           `json:"bucket"`
	Status           string           `json:"status"`
	SourcePath       string           `json:"source_path"`
	SourceLine       int              `json:"source_line"`
	SourceBucket     string           `json:"source_bucket"`
	DueAt            string           `json:"due_at"`
	Tags             []string         `json:"tags"`
	AgentSuggestion  string           `json:"agent_suggestion"`
	NoteText         string           `json:"note_text"`
	Prerequisite     string           `json:"prerequisite"`
	PlannedAt        string           `json:"planned_at"`
	PreviousBucket   string           `json:"previous_bucket"`
	PreviousDueAt    string           `json:"previous_due_at"`
	PreviousStatus   string           `json:"previous_status"`
	EndedAt          string           `json:"ended_at"`
	RelatedResources []map[string]any `json:"related_resources"`
	LinkedTaskID     string           `json:"linked_task_id"`
	CreatedAt        string           `json:"created_at"`
	UpdatedAt        string           `json:"updated_at"`
}

type bootstrapSeedTodoRule struct {
	RuleID               string `json:"rule_id"`
	ItemID               string `json:"item_id"`
	RuleType             string `json:"rule_type"`
	CronExpr             string `json:"cron_expr"`
	IntervalValue        int    `json:"interval_value"`
	IntervalUnit         string `json:"interval_unit"`
	ReminderStrategy     string `json:"reminder_strategy"`
	Enabled              bool   `json:"enabled"`
	RepeatRuleText       string `json:"repeat_rule_text"`
	NextOccurrenceAt     string `json:"next_occurrence_at"`
	RecentInstanceStatus string `json:"recent_instance_status"`
	EffectiveScope       string `json:"effective_scope"`
	CreatedAt            string `json:"created_at"`
	UpdatedAt            string `json:"updated_at"`
}

type bootstrapSeedTaskState struct {
	Tasks []bootstrapSeedTask `json:"tasks"`
}

type bootstrapSeedTask struct {
	TaskID            string                  `json:"task_id"`
	SessionID         string                  `json:"session_id"`
	RunID             string                  `json:"run_id"`
	Title             string                  `json:"title"`
	SourceType        string                  `json:"source_type"`
	Status            string                  `json:"status"`
	Intent            map[string]any          `json:"intent"`
	PreferredDelivery string                  `json:"preferred_delivery"`
	FallbackDelivery  string                  `json:"fallback_delivery"`
	CurrentStep       string                  `json:"current_step"`
	RiskLevel         string                  `json:"risk_level"`
	StartedAt         string                  `json:"started_at"`
	UpdatedAt         string                  `json:"updated_at"`
	FinishedAt        string                  `json:"finished_at"`
	Timeline          []bootstrapSeedTaskStep `json:"timeline"`
	BubbleMessage     map[string]any          `json:"bubble_message"`
	DeliveryResult    map[string]any          `json:"delivery_result"`
	Artifacts         []map[string]any        `json:"artifacts"`
	AuditRecords      []map[string]any        `json:"audit_records"`
	MirrorReferences  []map[string]any        `json:"mirror_references"`
	SecuritySummary   map[string]any          `json:"security_summary"`
	ApprovalRequest   map[string]any          `json:"approval_request"`
	PendingExecution  map[string]any          `json:"pending_execution"`
	Authorization     map[string]any          `json:"authorization"`
	ImpactScope       map[string]any          `json:"impact_scope"`
	TokenUsage        map[string]any          `json:"token_usage"`
	LatestEvent       map[string]any          `json:"latest_event"`
	LatestToolCall    map[string]any          `json:"latest_tool_call"`
	LoopStopReason    string                  `json:"loop_stop_reason"`
	SteeringMessages  []string                `json:"steering_messages"`
	CurrentStepStatus string                  `json:"current_step_status"`
	Events            []bootstrapSeedEvent    `json:"events"`
	DeliveryRecordID  string                  `json:"delivery_result_id"`
}

type bootstrapSeedTaskStep struct {
	StepID        string `json:"step_id"`
	TaskID        string `json:"task_id"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	OrderIndex    int    `json:"order_index"`
	InputSummary  string `json:"input_summary"`
	OutputSummary string `json:"output_summary"`
}

type bootstrapSeedEvent struct {
	EventID   string         `json:"event_id"`
	RunID     string         `json:"run_id"`
	TaskID    string         `json:"task_id"`
	StepID    string         `json:"step_id"`
	Type      string         `json:"type"`
	Level     string         `json:"level"`
	Payload   map[string]any `json:"payload"`
	CreatedAt string         `json:"created_at"`
}

type bootstrapSeedMemoryState struct {
	Summaries     []bootstrapSeedMemorySummary   `json:"summaries"`
	RetrievalHits []bootstrapSeedMemoryReference `json:"retrieval_hits"`
}

type bootstrapSeedMemorySummary struct {
	MemorySummaryID string `json:"memory_summary_id"`
	TaskID          string `json:"task_id"`
	RunID           string `json:"run_id"`
	Summary         string `json:"summary"`
	CreatedAt       string `json:"created_at"`
}

type bootstrapSeedMemoryReference struct {
	RetrievalHitID string  `json:"retrieval_hit_id"`
	TaskID         string  `json:"task_id"`
	RunID          string  `json:"run_id"`
	MemoryID       string  `json:"memory_id"`
	Score          float64 `json:"score"`
	Source         string  `json:"source"`
	Summary        string  `json:"summary"`
	CreatedAt      string  `json:"created_at"`
}

type bootstrapSeedGovernanceState struct {
	ApprovalRequests     []bootstrapSeedApprovalRequest     `json:"approval_requests"`
	AuthorizationRecords []bootstrapSeedAuthorizationRecord `json:"authorization_records"`
	AuditRecords         []bootstrapSeedAuditRecord         `json:"audit_records"`
	RecoveryPoints       []bootstrapSeedRecoveryPoint       `json:"recovery_points"`
}

type bootstrapSeedApprovalRequest struct {
	ApprovalID    string         `json:"approval_id"`
	TaskID        string         `json:"task_id"`
	OperationName string         `json:"operation_name"`
	RiskLevel     string         `json:"risk_level"`
	TargetObject  string         `json:"target_object"`
	Reason        string         `json:"reason"`
	Status        string         `json:"status"`
	ImpactScope   map[string]any `json:"impact_scope"`
	CreatedAt     string         `json:"created_at"`
	UpdatedAt     string         `json:"updated_at"`
}

type bootstrapSeedAuthorizationRecord struct {
	AuthorizationRecordID string `json:"authorization_record_id"`
	TaskID                string `json:"task_id"`
	ApprovalID            string `json:"approval_id"`
	Decision              string `json:"decision"`
	Operator              string `json:"operator"`
	RememberRule          bool   `json:"remember_rule"`
	CreatedAt             string `json:"created_at"`
}

type bootstrapSeedAuditRecord struct {
	AuditID   string `json:"audit_id"`
	TaskID    string `json:"task_id"`
	Type      string `json:"type"`
	Action    string `json:"action"`
	Summary   string `json:"summary"`
	Target    string `json:"target"`
	Result    string `json:"result"`
	CreatedAt string `json:"created_at"`
}

type bootstrapSeedRecoveryPoint struct {
	RecoveryPointID string   `json:"recovery_point_id"`
	TaskID          string   `json:"task_id"`
	Summary         string   `json:"summary"`
	CreatedAt       string   `json:"created_at"`
	Objects         []string `json:"objects"`
}

type bootstrapSeedSettingsState struct {
	Settings map[string]any `json:"settings"`
}

type bootstrapSeedMarker struct {
	SeedVersion string `json:"seed_version"`
	ImportedAt  string `json:"imported_at"`
}

func loadBootstrapSeed(seedDir string) (*bootstrapSeedBundle, error) {
	// Load the bundled seed manifest plus all declared JSON payloads from the
	// packaged resource directory. Missing seed data is treated as an optional
	// capability so development runs can still boot without installer assets.
	seedDir = strings.TrimSpace(seedDir)
	if seedDir == "" {
		return nil, nil
	}

	manifestPath := filepath.Join(seedDir, "manifest.json")
	if _, err := os.Stat(manifestPath); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("stat bootstrap seed manifest: %w", err)
	}

	manifest, err := readSeedJSON[bootstrapSeedManifest](manifestPath)
	if err != nil {
		return nil, err
	}
	manifest = normalizeBootstrapSeedManifest(manifest)

	todoState, err := readSeedJSON[bootstrapSeedTodoState](filepath.Join(seedDir, manifest.TodoStateFile))
	if err != nil {
		return nil, err
	}
	taskState, err := readSeedJSON[bootstrapSeedTaskState](filepath.Join(seedDir, manifest.TasksFile))
	if err != nil {
		return nil, err
	}
	memoryState, err := readSeedJSON[bootstrapSeedMemoryState](filepath.Join(seedDir, manifest.MemoryFile))
	if err != nil {
		return nil, err
	}
	governanceState, err := readSeedJSON[bootstrapSeedGovernanceState](filepath.Join(seedDir, manifest.GovernanceFile))
	if err != nil {
		return nil, err
	}
	settingsState, err := readSeedJSON[bootstrapSeedSettingsState](filepath.Join(seedDir, manifest.SettingsFile))
	if err != nil {
		return nil, err
	}

	return &bootstrapSeedBundle{
		Manifest:   manifest,
		TodoState:  todoState,
		Tasks:      taskState,
		Memory:     memoryState,
		Governance: governanceState,
		Settings:   settingsState,
	}, nil
}

func normalizeBootstrapSeedManifest(manifest bootstrapSeedManifest) bootstrapSeedManifest {
	if strings.TrimSpace(manifest.TodoStateFile) == "" {
		manifest.TodoStateFile = "todo_state.json"
	}
	if strings.TrimSpace(manifest.TasksFile) == "" {
		manifest.TasksFile = "tasks.json"
	}
	if strings.TrimSpace(manifest.MemoryFile) == "" {
		manifest.MemoryFile = "memory.json"
	}
	if strings.TrimSpace(manifest.GovernanceFile) == "" {
		manifest.GovernanceFile = "governance.json"
	}
	if strings.TrimSpace(manifest.SettingsFile) == "" {
		manifest.SettingsFile = "settings.json"
	}
	if strings.TrimSpace(manifest.WorkspaceDir) == "" {
		manifest.WorkspaceDir = "workspace"
	}
	return manifest
}

func applyBootstrapSeedIfNeeded(cfg config.Config, storageService *storage.Service) (*bootstrapSeedBundle, error) {
	// Persisted seed data imports only once per user data directory so first-run
	// workspace content is available without overwriting later user changes.
	seedBundle, err := loadBootstrapSeed(cfg.SeedDir)
	if err != nil || seedBundle == nil {
		return seedBundle, err
	}
	if hasBootstrapSeedMarker(cfg.DataDir) {
		return seedBundle, nil
	}

	workspaceSeedRoot := filepath.Join(cfg.SeedDir, seedBundle.Manifest.WorkspaceDir)
	if err := copySeedWorkspace(workspaceSeedRoot, cfg.WorkspaceRoot); err != nil {
		return nil, err
	}

	ctx := context.Background()
	todoItems, todoRules, err := seedTodoStateToRecords(seedBundle.TodoState)
	if err != nil {
		return nil, err
	}
	if err := storageService.TodoStore().ReplaceTodoState(ctx, todoItems, todoRules); err != nil {
		return nil, fmt.Errorf("replace seeded todo state: %w", err)
	}

	artifacts := make([]storage.ArtifactRecord, 0)
	for _, task := range seedBundle.Tasks.Tasks {
		taskRecord, err := seedTaskToTaskRunRecord(task)
		if err != nil {
			return nil, err
		}
		if err := storageService.TaskRunStore().SaveTaskRun(ctx, taskRecord); err != nil {
			return nil, fmt.Errorf("save seeded task run %s: %w", task.TaskID, err)
		}
		if storageService.LoopRuntimeStore() != nil {
			if err := storageService.LoopRuntimeStore().SaveRun(ctx, seedTaskToRunRecord(task)); err != nil {
				return nil, fmt.Errorf("save seeded run %s: %w", task.RunID, err)
			}
			eventRecords, err := seedTaskToEventRecords(task)
			if err != nil {
				return nil, err
			}
			if len(eventRecords) > 0 {
				if err := storageService.LoopRuntimeStore().SaveEvents(ctx, eventRecords); err != nil {
					return nil, fmt.Errorf("save seeded events for %s: %w", task.TaskID, err)
				}
			}
			deliveryRecord, ok, err := seedTaskToDeliveryResultRecord(task)
			if err != nil {
				return nil, err
			}
			if ok {
				if err := storageService.LoopRuntimeStore().SaveDeliveryResult(ctx, deliveryRecord); err != nil {
					return nil, fmt.Errorf("save seeded delivery result for %s: %w", task.TaskID, err)
				}
			}
		}
		artifactRecords, err := seedTaskToArtifactRecords(task)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifactRecords...)
	}
	if len(artifacts) > 0 && storageService.ArtifactStore() != nil {
		if err := storageService.ArtifactStore().SaveArtifacts(ctx, artifacts); err != nil {
			return nil, fmt.Errorf("save seeded artifacts: %w", err)
		}
	}

	memorySummaries, retrievalHits := seedMemoryStateToRecords(seedBundle.Memory)
	for _, summary := range memorySummaries {
		if err := storageService.MemoryStore().SaveSummary(ctx, summary); err != nil {
			return nil, fmt.Errorf("save seeded memory summary %s: %w", summary.MemorySummaryID, err)
		}
	}
	if len(retrievalHits) > 0 {
		if err := storageService.MemoryStore().SaveRetrievalHits(ctx, retrievalHits); err != nil {
			return nil, fmt.Errorf("save seeded retrieval hits: %w", err)
		}
	}

	if err := importSeedGovernance(ctx, storageService, seedBundle.Governance); err != nil {
		return nil, err
	}

	if err := writeBootstrapSeedMarker(cfg.DataDir, seedBundle.Manifest.SeedVersion); err != nil {
		return nil, err
	}

	return seedBundle, nil
}

func applyBootstrapRuntimeSettings(runEngine *runengine.Engine, seedBundle *bootstrapSeedBundle) {
	if runEngine == nil || seedBundle == nil || len(seedBundle.Settings.Settings) == 0 {
		return
	}

	// Settings currently live in the runengine runtime rather than a dedicated
	// persisted store, so the bundled seed acts as the packaged default snapshot
	// every time the desktop shell boots the local service.
	runEngine.UpdateSettings(seedBundle.Settings.Settings)
	if taskAutomation, ok := seedBundle.Settings.Settings["task_automation"].(map[string]any); ok {
		runEngine.UpdateInspectorConfig(taskAutomation)
	}
}

func hasBootstrapSeedMarker(dataDir string) bool {
	markerPath := filepath.Join(strings.TrimSpace(dataDir), seedMarkerRelativePath)
	_, err := os.Stat(markerPath)
	return err == nil
}

func writeBootstrapSeedMarker(dataDir, seedVersion string) error {
	markerPath := filepath.Join(strings.TrimSpace(dataDir), seedMarkerRelativePath)
	if err := os.MkdirAll(filepath.Dir(markerPath), 0o755); err != nil {
		return fmt.Errorf("prepare bootstrap seed marker directory: %w", err)
	}
	payload, err := json.MarshalIndent(bootstrapSeedMarker{
		SeedVersion: strings.TrimSpace(seedVersion),
		ImportedAt:  time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode bootstrap seed marker: %w", err)
	}
	if err := os.WriteFile(markerPath, append(payload, '\n'), 0o644); err != nil {
		return fmt.Errorf("write bootstrap seed marker: %w", err)
	}
	return nil
}

func copySeedWorkspace(sourceRoot, targetRoot string) error {
	// The bundled workspace files must land in the user data workspace rather
	// than the installer directory so later edits remain writable and isolated.
	if _, err := os.Stat(sourceRoot); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("stat bootstrap workspace seed: %w", err)
	}

	return filepath.WalkDir(sourceRoot, func(currentPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relPath, err := filepath.Rel(sourceRoot, currentPath)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(targetRoot, relPath)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		content, err := os.ReadFile(currentPath)
		if err != nil {
			return fmt.Errorf("read bootstrap workspace file %s: %w", currentPath, err)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("prepare bootstrap workspace file %s: %w", targetPath, err)
		}
		if err := os.WriteFile(targetPath, content, 0o644); err != nil {
			return fmt.Errorf("write bootstrap workspace file %s: %w", targetPath, err)
		}
		return nil
	})
}

func seedTodoStateToRecords(state bootstrapSeedTodoState) ([]storage.TodoItemRecord, []storage.RecurringRuleRecord, error) {
	items := make([]storage.TodoItemRecord, 0, len(state.Items))
	for _, item := range state.Items {
		relatedResourcesJSON, err := marshalOptionalJSON(item.RelatedResources)
		if err != nil {
			return nil, nil, fmt.Errorf("encode seeded related resources for %s: %w", item.ItemID, err)
		}
		tagsJSON, err := marshalOptionalJSON(item.Tags)
		if err != nil {
			return nil, nil, fmt.Errorf("encode seeded tags for %s: %w", item.ItemID, err)
		}
		items = append(items, storage.TodoItemRecord{
			ItemID:               strings.TrimSpace(item.ItemID),
			Title:                strings.TrimSpace(item.Title),
			Bucket:               strings.TrimSpace(item.Bucket),
			Status:               strings.TrimSpace(item.Status),
			SourcePath:           strings.TrimSpace(item.SourcePath),
			SourceLine:           item.SourceLine,
			SourceBucket:         strings.TrimSpace(item.SourceBucket),
			DueAt:                strings.TrimSpace(item.DueAt),
			TagsJSON:             tagsJSON,
			AgentSuggestion:      strings.TrimSpace(item.AgentSuggestion),
			NoteText:             strings.TrimSpace(item.NoteText),
			Prerequisite:         strings.TrimSpace(item.Prerequisite),
			PlannedAt:            strings.TrimSpace(item.PlannedAt),
			PreviousBucket:       strings.TrimSpace(item.PreviousBucket),
			PreviousDueAt:        strings.TrimSpace(item.PreviousDueAt),
			PreviousStatus:       strings.TrimSpace(item.PreviousStatus),
			EndedAt:              strings.TrimSpace(item.EndedAt),
			RelatedResourcesJSON: relatedResourcesJSON,
			LinkedTaskID:         strings.TrimSpace(item.LinkedTaskID),
			CreatedAt:            strings.TrimSpace(item.CreatedAt),
			UpdatedAt:            strings.TrimSpace(item.UpdatedAt),
		})
	}

	rules := make([]storage.RecurringRuleRecord, 0, len(state.Rules))
	for _, rule := range state.Rules {
		rules = append(rules, storage.RecurringRuleRecord{
			RuleID:               strings.TrimSpace(rule.RuleID),
			ItemID:               strings.TrimSpace(rule.ItemID),
			RuleType:             strings.TrimSpace(rule.RuleType),
			CronExpr:             strings.TrimSpace(rule.CronExpr),
			IntervalValue:        rule.IntervalValue,
			IntervalUnit:         strings.TrimSpace(rule.IntervalUnit),
			ReminderStrategy:     strings.TrimSpace(rule.ReminderStrategy),
			Enabled:              rule.Enabled,
			RepeatRuleText:       strings.TrimSpace(rule.RepeatRuleText),
			NextOccurrenceAt:     strings.TrimSpace(rule.NextOccurrenceAt),
			RecentInstanceStatus: strings.TrimSpace(rule.RecentInstanceStatus),
			EffectiveScope:       strings.TrimSpace(rule.EffectiveScope),
			CreatedAt:            strings.TrimSpace(rule.CreatedAt),
			UpdatedAt:            strings.TrimSpace(rule.UpdatedAt),
		})
	}

	return items, rules, nil
}

func seedTaskToTaskRunRecord(task bootstrapSeedTask) (storage.TaskRunRecord, error) {
	startedAt, err := parseSeedTime(task.StartedAt)
	if err != nil {
		return storage.TaskRunRecord{}, fmt.Errorf("parse started_at for seeded task %s: %w", task.TaskID, err)
	}
	updatedAt, err := parseSeedTime(task.UpdatedAt)
	if err != nil {
		return storage.TaskRunRecord{}, fmt.Errorf("parse updated_at for seeded task %s: %w", task.TaskID, err)
	}
	finishedAt, err := parseOptionalSeedTime(task.FinishedAt)
	if err != nil {
		return storage.TaskRunRecord{}, fmt.Errorf("parse finished_at for seeded task %s: %w", task.TaskID, err)
	}

	timeline := make([]storage.TaskStepSnapshot, 0, len(task.Timeline))
	for _, step := range task.Timeline {
		timeline = append(timeline, storage.TaskStepSnapshot{
			StepID:        strings.TrimSpace(step.StepID),
			TaskID:        firstNonEmptySeed(step.TaskID, task.TaskID),
			Name:          strings.TrimSpace(step.Name),
			Status:        strings.TrimSpace(step.Status),
			OrderIndex:    step.OrderIndex,
			InputSummary:  strings.TrimSpace(step.InputSummary),
			OutputSummary: strings.TrimSpace(step.OutputSummary),
		})
	}

	return storage.TaskRunRecord{
		TaskID:            strings.TrimSpace(task.TaskID),
		SessionID:         strings.TrimSpace(task.SessionID),
		RunID:             strings.TrimSpace(task.RunID),
		Title:             strings.TrimSpace(task.Title),
		SourceType:        strings.TrimSpace(task.SourceType),
		Status:            strings.TrimSpace(task.Status),
		Intent:            cloneSeedMap(task.Intent),
		PreferredDelivery: strings.TrimSpace(task.PreferredDelivery),
		FallbackDelivery:  strings.TrimSpace(task.FallbackDelivery),
		CurrentStep:       strings.TrimSpace(task.CurrentStep),
		RiskLevel:         strings.TrimSpace(task.RiskLevel),
		StartedAt:         startedAt,
		UpdatedAt:         updatedAt,
		FinishedAt:        finishedAt,
		Timeline:          timeline,
		BubbleMessage:     cloneSeedMap(task.BubbleMessage),
		DeliveryResult:    cloneSeedMap(task.DeliveryResult),
		Artifacts:         cloneSeedMapSlice(task.Artifacts),
		AuditRecords:      cloneSeedMapSlice(task.AuditRecords),
		MirrorReferences:  cloneSeedMapSlice(task.MirrorReferences),
		SecuritySummary:   cloneSeedMap(task.SecuritySummary),
		ApprovalRequest:   cloneSeedMap(task.ApprovalRequest),
		PendingExecution:  cloneSeedMap(task.PendingExecution),
		Authorization:     cloneSeedMap(task.Authorization),
		ImpactScope:       cloneSeedMap(task.ImpactScope),
		TokenUsage:        cloneSeedMap(task.TokenUsage),
		LatestEvent:       cloneSeedMap(task.LatestEvent),
		LatestToolCall:    cloneSeedMap(task.LatestToolCall),
		LoopStopReason:    strings.TrimSpace(task.LoopStopReason),
		SteeringMessages:  append([]string(nil), task.SteeringMessages...),
		CurrentStepStatus: strings.TrimSpace(task.CurrentStepStatus),
	}, nil
}

func seedTaskToRunRecord(task bootstrapSeedTask) storage.RunRecord {
	intentName := ""
	if task.Intent != nil {
		intentName = strings.TrimSpace(stringValueFromSeedMap(task.Intent, "name"))
	}
	return storage.RunRecord{
		RunID:      strings.TrimSpace(task.RunID),
		TaskID:     strings.TrimSpace(task.TaskID),
		SessionID:  strings.TrimSpace(task.SessionID),
		Status:     strings.TrimSpace(task.Status),
		IntentName: intentName,
		StartedAt:  strings.TrimSpace(task.StartedAt),
		UpdatedAt:  strings.TrimSpace(task.UpdatedAt),
		FinishedAt: strings.TrimSpace(task.FinishedAt),
		StopReason: strings.TrimSpace(task.LoopStopReason),
	}
}

func seedTaskToEventRecords(task bootstrapSeedTask) ([]storage.EventRecord, error) {
	records := make([]storage.EventRecord, 0, len(task.Events))
	for _, event := range task.Events {
		payloadJSON, err := marshalOptionalJSON(event.Payload)
		if err != nil {
			return nil, fmt.Errorf("encode seeded event payload %s: %w", event.EventID, err)
		}
		records = append(records, storage.EventRecord{
			EventID:     strings.TrimSpace(event.EventID),
			RunID:       firstNonEmptySeed(event.RunID, task.RunID),
			TaskID:      firstNonEmptySeed(event.TaskID, task.TaskID),
			StepID:      strings.TrimSpace(event.StepID),
			Type:        strings.TrimSpace(event.Type),
			Level:       strings.TrimSpace(event.Level),
			PayloadJSON: payloadJSON,
			CreatedAt:   strings.TrimSpace(event.CreatedAt),
		})
	}
	return records, nil
}

func seedTaskToDeliveryResultRecord(task bootstrapSeedTask) (storage.DeliveryResultRecord, bool, error) {
	if len(task.DeliveryResult) == 0 {
		return storage.DeliveryResultRecord{}, false, nil
	}
	payloadJSON, err := marshalOptionalJSON(task.DeliveryResult["payload"])
	if err != nil {
		return storage.DeliveryResultRecord{}, false, fmt.Errorf("encode seeded delivery payload for %s: %w", task.TaskID, err)
	}
	createdAt := firstNonEmptySeed(task.FinishedAt, task.UpdatedAt)
	return storage.DeliveryResultRecord{
		DeliveryResultID: firstNonEmptySeed(strings.TrimSpace(task.DeliveryRecordID), fmt.Sprintf("delivery_%s", strings.TrimSpace(task.TaskID))),
		TaskID:           strings.TrimSpace(task.TaskID),
		Type:             strings.TrimSpace(stringValueFromSeedMap(task.DeliveryResult, "type")),
		Title:            strings.TrimSpace(stringValueFromSeedMap(task.DeliveryResult, "title")),
		PayloadJSON:      payloadJSON,
		PreviewText:      strings.TrimSpace(stringValueFromSeedMap(task.DeliveryResult, "preview_text")),
		CreatedAt:        createdAt,
	}, true, nil
}

func seedTaskToArtifactRecords(task bootstrapSeedTask) ([]storage.ArtifactRecord, error) {
	records := make([]storage.ArtifactRecord, 0, len(task.Artifacts))
	for _, artifact := range task.Artifacts {
		deliveryPayloadJSON, err := marshalOptionalJSON(artifact["delivery_payload"])
		if err != nil {
			return nil, fmt.Errorf("encode seeded artifact payload for %s: %w", task.TaskID, err)
		}
		records = append(records, storage.ArtifactRecord{
			ArtifactID:          strings.TrimSpace(stringValueFromSeedMap(artifact, "artifact_id")),
			TaskID:              firstNonEmptySeed(strings.TrimSpace(stringValueFromSeedMap(artifact, "task_id")), task.TaskID),
			ArtifactType:        strings.TrimSpace(stringValueFromSeedMap(artifact, "artifact_type")),
			Title:               strings.TrimSpace(stringValueFromSeedMap(artifact, "title")),
			Path:                strings.TrimSpace(stringValueFromSeedMap(artifact, "path")),
			MimeType:            strings.TrimSpace(stringValueFromSeedMap(artifact, "mime_type")),
			DeliveryType:        strings.TrimSpace(stringValueFromSeedMap(artifact, "delivery_type")),
			DeliveryPayloadJSON: deliveryPayloadJSON,
			CreatedAt:           firstNonEmptySeed(strings.TrimSpace(stringValueFromSeedMap(artifact, "created_at")), task.UpdatedAt),
		})
	}
	return records, nil
}

func seedMemoryStateToRecords(state bootstrapSeedMemoryState) ([]storage.MemorySummaryRecord, []storage.MemoryRetrievalRecord) {
	summaries := make([]storage.MemorySummaryRecord, 0, len(state.Summaries))
	for _, summary := range state.Summaries {
		summaries = append(summaries, storage.MemorySummaryRecord{
			MemorySummaryID: strings.TrimSpace(summary.MemorySummaryID),
			TaskID:          strings.TrimSpace(summary.TaskID),
			RunID:           strings.TrimSpace(summary.RunID),
			Summary:         strings.TrimSpace(summary.Summary),
			CreatedAt:       strings.TrimSpace(summary.CreatedAt),
		})
	}

	hits := make([]storage.MemoryRetrievalRecord, 0, len(state.RetrievalHits))
	for _, hit := range state.RetrievalHits {
		hits = append(hits, storage.MemoryRetrievalRecord{
			RetrievalHitID: strings.TrimSpace(hit.RetrievalHitID),
			TaskID:         strings.TrimSpace(hit.TaskID),
			RunID:          strings.TrimSpace(hit.RunID),
			MemoryID:       strings.TrimSpace(hit.MemoryID),
			Score:          hit.Score,
			Source:         strings.TrimSpace(hit.Source),
			Summary:        strings.TrimSpace(hit.Summary),
			CreatedAt:      strings.TrimSpace(hit.CreatedAt),
		})
	}

	return summaries, hits
}

func importSeedGovernance(ctx context.Context, storageService *storage.Service, state bootstrapSeedGovernanceState) error {
	for _, request := range state.ApprovalRequests {
		impactScopeJSON, err := marshalOptionalJSON(request.ImpactScope)
		if err != nil {
			return fmt.Errorf("encode seeded approval impact scope %s: %w", request.ApprovalID, err)
		}
		if err := storageService.ApprovalRequestStore().WriteApprovalRequest(ctx, storage.ApprovalRequestRecord{
			ApprovalID:      strings.TrimSpace(request.ApprovalID),
			TaskID:          strings.TrimSpace(request.TaskID),
			OperationName:   strings.TrimSpace(request.OperationName),
			RiskLevel:       strings.TrimSpace(request.RiskLevel),
			TargetObject:    strings.TrimSpace(request.TargetObject),
			Reason:          strings.TrimSpace(request.Reason),
			Status:          strings.TrimSpace(request.Status),
			ImpactScopeJSON: impactScopeJSON,
			CreatedAt:       strings.TrimSpace(request.CreatedAt),
			UpdatedAt:       strings.TrimSpace(request.UpdatedAt),
		}); err != nil {
			return fmt.Errorf("write seeded approval request %s: %w", request.ApprovalID, err)
		}
	}

	for _, record := range state.AuthorizationRecords {
		if err := storageService.AuthorizationRecordStore().WriteAuthorizationRecord(ctx, storage.AuthorizationRecordRecord{
			AuthorizationRecordID: strings.TrimSpace(record.AuthorizationRecordID),
			TaskID:                strings.TrimSpace(record.TaskID),
			ApprovalID:            strings.TrimSpace(record.ApprovalID),
			Decision:              strings.TrimSpace(record.Decision),
			Operator:              strings.TrimSpace(record.Operator),
			RememberRule:          record.RememberRule,
			CreatedAt:             strings.TrimSpace(record.CreatedAt),
		}); err != nil {
			return fmt.Errorf("write seeded authorization record %s: %w", record.AuthorizationRecordID, err)
		}
	}

	for _, record := range state.AuditRecords {
		if err := storageService.AuditStore().WriteAuditRecord(ctx, audit.Record{
			AuditID:   strings.TrimSpace(record.AuditID),
			TaskID:    strings.TrimSpace(record.TaskID),
			Type:      strings.TrimSpace(record.Type),
			Action:    strings.TrimSpace(record.Action),
			Summary:   strings.TrimSpace(record.Summary),
			Target:    strings.TrimSpace(record.Target),
			Result:    strings.TrimSpace(record.Result),
			CreatedAt: strings.TrimSpace(record.CreatedAt),
		}); err != nil {
			return fmt.Errorf("write seeded audit record %s: %w", record.AuditID, err)
		}
	}

	for _, point := range state.RecoveryPoints {
		if err := storageService.RecoveryPointStore().WriteRecoveryPoint(ctx, checkpoint.RecoveryPoint{
			RecoveryPointID: strings.TrimSpace(point.RecoveryPointID),
			TaskID:          strings.TrimSpace(point.TaskID),
			Summary:         strings.TrimSpace(point.Summary),
			CreatedAt:       strings.TrimSpace(point.CreatedAt),
			Objects:         append([]string(nil), point.Objects...),
		}); err != nil {
			return fmt.Errorf("write seeded recovery point %s: %w", point.RecoveryPointID, err)
		}
	}

	return nil
}

func parseSeedTime(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, errors.New("time value is required")
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err == nil {
		return parsed, nil
	}
	return time.Parse(time.RFC3339Nano, trimmed)
}

func parseOptionalSeedTime(value string) (*time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := parseSeedTime(trimmed)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func readSeedJSON[T any](path string) (T, error) {
	var value T
	payload, err := os.ReadFile(path)
	if err != nil {
		return value, fmt.Errorf("read bootstrap seed %s: %w", path, err)
	}
	if err := json.Unmarshal(payload, &value); err != nil {
		return value, fmt.Errorf("decode bootstrap seed %s: %w", path, err)
	}
	return value, nil
}

func marshalOptionalJSON(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	if string(payload) == "null" {
		return "", nil
	}
	return string(payload), nil
}

func cloneSeedMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	var cloned map[string]any
	if err := json.Unmarshal(payload, &cloned); err != nil {
		return map[string]any{}
	}
	return cloned
}

func cloneSeedMapSlice(values []map[string]any) []map[string]any {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]map[string]any, 0, len(values))
	for _, value := range values {
		cloned = append(cloned, cloneSeedMap(value))
	}
	return cloned
}

func stringValueFromSeedMap(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func firstNonEmptySeed(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
