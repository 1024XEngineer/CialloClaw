package bootstrap

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

func TestApplyBootstrapSeedIfNeededImportsPersistentStateAndRuntimeDefaults(t *testing.T) {
	baseDir := t.TempDir()
	seedDir := filepath.Join(baseDir, "seed")
	dataDir := filepath.Join(baseDir, "app-data")
	if err := writeBootstrapSeedFixture(seedDir); err != nil {
		t.Fatalf("write bootstrap seed fixture: %v", err)
	}

	cfg := config.Load(config.LoadOptions{DataDir: dataDir, SeedDir: seedDir})
	storageService := storage.NewService(platform.NewLocalStorageAdapter(cfg.DatabasePath))
	defer func() {
		if err := storageService.Close(); err != nil {
			t.Fatalf("close storage service: %v", err)
		}
	}()

	seedBundle, err := applyBootstrapSeedIfNeeded(cfg, storageService)
	if err != nil {
		t.Fatalf("apply bootstrap seed: %v", err)
	}
	if seedBundle == nil {
		t.Fatal("expected bootstrap seed bundle to be loaded")
	}

	workspaceReport := filepath.Join(cfg.WorkspaceRoot, "reports", "seed-report.md")
	if _, err := os.Stat(workspaceReport); err != nil {
		t.Fatalf("expected seeded workspace report to exist: %v", err)
	}

	items, rules, err := storageService.TodoStore().LoadTodoState(context.Background())
	if err != nil {
		t.Fatalf("load seeded todo state: %v", err)
	}
	if len(items) != 1 || items[0].ItemID != "todo_seed_001" {
		t.Fatalf("expected one seeded todo item, got %+v", items)
	}
	if len(rules) != 1 || rules[0].RuleID != "rule_seed_001" {
		t.Fatalf("expected one seeded todo rule, got %+v", rules)
	}

	taskRuns, err := storageService.TaskRunStore().LoadTaskRuns(context.Background())
	if err != nil {
		t.Fatalf("load seeded task runs: %v", err)
	}
	if len(taskRuns) != 2 {
		t.Fatalf("expected two seeded task runs, got %+v", taskRuns)
	}

	summaries, err := storageService.MemoryStore().ListRecentSummaries(context.Background(), 10)
	if err != nil {
		t.Fatalf("load seeded memory summaries: %v", err)
	}
	if len(summaries) != 1 || summaries[0].MemorySummaryID != "mem_seed_001" {
		t.Fatalf("expected one seeded memory summary, got %+v", summaries)
	}

	approvalRecords, approvalTotal, err := storageService.ApprovalRequestStore().ListPendingApprovalRequests(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("load seeded approval requests: %v", err)
	}
	if approvalTotal != 1 || len(approvalRecords) != 1 || approvalRecords[0].ApprovalID != "approval_seed_001" {
		t.Fatalf("expected one seeded pending approval request, got total=%d records=%+v", approvalTotal, approvalRecords)
	}

	auditRecords, auditTotal, err := storageService.AuditStore().ListAuditRecords(context.Background(), "", 10, 0)
	if err != nil {
		t.Fatalf("load seeded audit records: %v", err)
	}
	if auditTotal != 1 || len(auditRecords) != 1 || auditRecords[0].AuditID != "audit_seed_001" {
		t.Fatalf("expected one seeded audit record, got total=%d records=%+v", auditTotal, auditRecords)
	}

	recoveryPoints, recoveryTotal, err := storageService.RecoveryPointStore().ListRecoveryPoints(context.Background(), "", 10, 0)
	if err != nil {
		t.Fatalf("load seeded recovery points: %v", err)
	}
	if recoveryTotal != 1 || len(recoveryPoints) != 1 || recoveryPoints[0].RecoveryPointID != "rp_seed_001" {
		t.Fatalf("expected one seeded recovery point, got total=%d records=%+v", recoveryTotal, recoveryPoints)
	}

	if _, err := applyBootstrapSeedIfNeeded(cfg, storageService); err != nil {
		t.Fatalf("reapply bootstrap seed should skip via marker: %v", err)
	}
	approvalRecords, approvalTotal, err = storageService.ApprovalRequestStore().ListPendingApprovalRequests(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("reload approval requests after marker skip: %v", err)
	}
	if approvalTotal != 1 || len(approvalRecords) != 1 {
		t.Fatalf("expected marker to prevent duplicate approval imports, got total=%d records=%+v", approvalTotal, approvalRecords)
	}

	runEngine, err := runengine.NewEngineWithStore(storageService.TaskRunStore())
	if err != nil {
		t.Fatalf("create run engine for seeded settings: %v", err)
	}
	if err := runEngine.WithTodoStore(storageService.TodoStore()); err != nil {
		t.Fatalf("attach seeded todo store: %v", err)
	}
	applyBootstrapRuntimeSettings(runEngine, seedBundle)
	settings := runEngine.Settings()
	general, ok := settings["general"].(map[string]any)
	if !ok || general["language"] != "zh-CN" {
		t.Fatalf("expected seeded general settings, got %+v", settings)
	}
	inspector := runEngine.InspectorConfig()
	taskSources, ok := inspector["task_sources"].([]string)
	if !ok || len(taskSources) != 1 || taskSources[0] != "workspace/todos" {
		t.Fatalf("expected seeded inspector sources, got %+v", inspector)
	}
}

func writeBootstrapSeedFixture(seedDir string) error {
	if err := os.MkdirAll(filepath.Join(seedDir, "workspace", "reports"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(seedDir, "workspace", "todos"), 0o755); err != nil {
		return err
	}

	files := map[string]any{
		"manifest.json": bootstrapSeedManifest{
			SeedVersion:    "seed-test-v1",
			SeedName:       "seed_test",
			Description:    "bootstrap seed test fixture",
			TodoStateFile:  "todo_state.json",
			TasksFile:      "tasks.json",
			MemoryFile:     "memory.json",
			GovernanceFile: "governance.json",
			SettingsFile:   "settings.json",
			WorkspaceDir:   "workspace",
		},
		"todo_state.json": bootstrapSeedTodoState{
			Items: []bootstrapSeedTodoItem{{
				ItemID:          "todo_seed_001",
				Title:           "Seeded note",
				Bucket:          "upcoming",
				Status:          "normal",
				SourcePath:      "workspace/todos/inbox.md",
				SourceLine:      1,
				DueAt:           "2026-04-19T18:00:00Z",
				PlannedAt:       "2026-04-19T17:00:00Z",
				CreatedAt:       "2026-04-19T08:00:00Z",
				UpdatedAt:       "2026-04-19T08:10:00Z",
				AgentSuggestion: "Use the seeded report as the first reference.",
			}},
			Rules: []bootstrapSeedTodoRule{{
				RuleID:           "rule_seed_001",
				ItemID:           "todo_seed_001",
				RuleType:         "interval",
				IntervalValue:    1,
				IntervalUnit:     "week",
				ReminderStrategy: "default",
				Enabled:          true,
				RepeatRuleText:   "Every Monday 09:00",
				CreatedAt:        "2026-04-19T08:00:00Z",
				UpdatedAt:        "2026-04-19T08:00:00Z",
			}},
		},
		"tasks.json": bootstrapSeedTaskState{
			Tasks: []bootstrapSeedTask{
				{
					TaskID:            "task_seed_processing",
					SessionID:         "sess_seed",
					RunID:             "run_seed_processing",
					Title:             "Seeded processing task",
					SourceType:        "todo",
					Status:            "processing",
					Intent:            map[string]any{"name": "seed_processing"},
					PreferredDelivery: "bubble",
					FallbackDelivery:  "bubble",
					CurrentStep:       "validate",
					RiskLevel:         "green",
					StartedAt:         "2026-04-19T08:20:00Z",
					UpdatedAt:         "2026-04-19T08:30:00Z",
					Events: []bootstrapSeedEvent{{
						EventID:   "evt_seed_processing",
						RunID:     "run_seed_processing",
						TaskID:    "task_seed_processing",
						Type:      "run.progress",
						Level:     "info",
						Payload:   map[string]any{"summary": "seed processing"},
						CreatedAt: "2026-04-19T08:30:00Z",
					}},
				},
				{
					TaskID:            "task_seed_completed",
					SessionID:         "sess_seed",
					RunID:             "run_seed_completed",
					Title:             "Seeded completed task",
					SourceType:        "todo",
					Status:            "completed",
					Intent:            map[string]any{"name": "seed_completed"},
					PreferredDelivery: "workspace_document",
					FallbackDelivery:  "bubble",
					CurrentStep:       "delivery_ready",
					RiskLevel:         "green",
					StartedAt:         "2026-04-18T09:00:00Z",
					UpdatedAt:         "2026-04-18T09:20:00Z",
					FinishedAt:        "2026-04-18T09:20:00Z",
					DeliveryRecordID:  "delivery_seed_completed",
					DeliveryResult: map[string]any{
						"type":  "workspace_document",
						"title": "Seeded report",
						"payload": map[string]any{
							"path":    "workspace/reports/seed-report.md",
							"task_id": "task_seed_completed",
							"url":     nil,
						},
						"preview_text": "seeded report ready",
					},
					Artifacts: []map[string]any{{
						"artifact_id":   "art_seed_completed",
						"task_id":       "task_seed_completed",
						"artifact_type": "generated_doc",
						"title":         "seed-report.md",
						"path":          "workspace/reports/seed-report.md",
						"mime_type":     "text/markdown",
						"delivery_type": "workspace_document",
						"delivery_payload": map[string]any{
							"path":    "workspace/reports/seed-report.md",
							"task_id": "task_seed_completed",
						},
						"created_at": "2026-04-18T09:20:00Z",
					}},
					MirrorReferences: []map[string]any{{
						"memory_id": "mem_seed_001",
						"reason":    "Seeded memory reference",
						"summary":   "Seeded memory reference",
					}},
					Authorization: map[string]any{
						"authorization_record_id": "auth_seed_001",
						"task_id":                 "task_seed_completed",
						"approval_id":             "approval_seed_archived",
						"decision":                "allow_once",
						"operator":                "seed_test",
						"remember_rule":           false,
						"created_at":              "2026-04-18T09:10:00Z",
					},
					SecuritySummary: map[string]any{
						"security_status": "normal",
						"latest_restore_point": map[string]any{
							"recovery_point_id": "rp_seed_001",
							"task_id":           "task_seed_completed",
							"summary":           "Seed recovery point",
							"created_at":        "2026-04-18T09:05:00Z",
							"objects":           []string{"workspace/reports/seed-report.md"},
						},
					},
					Events: []bootstrapSeedEvent{{
						EventID:   "evt_seed_completed",
						RunID:     "run_seed_completed",
						TaskID:    "task_seed_completed",
						Type:      "run.completed",
						Level:     "info",
						Payload:   map[string]any{"summary": "seed completed"},
						CreatedAt: "2026-04-18T09:20:00Z",
					}},
				},
			},
		},
		"memory.json": bootstrapSeedMemoryState{
			Summaries: []bootstrapSeedMemorySummary{{
				MemorySummaryID: "mem_seed_001",
				TaskID:          "task_seed_completed",
				RunID:           "run_seed_completed",
				Summary:         "Seeded memory summary",
				CreatedAt:       "2026-04-18T09:20:00Z",
			}},
			RetrievalHits: []bootstrapSeedMemoryReference{{
				RetrievalHitID: "hit_seed_001",
				TaskID:         "task_seed_processing",
				RunID:          "run_seed_processing",
				MemoryID:       "mem_seed_001",
				Score:          0.9,
				Source:         "seed_test",
				Summary:        "Seeded memory retrieval",
				CreatedAt:      "2026-04-19T08:30:00Z",
			}},
		},
		"governance.json": bootstrapSeedGovernanceState{
			ApprovalRequests: []bootstrapSeedApprovalRequest{{
				ApprovalID:    "approval_seed_001",
				TaskID:        "task_seed_processing",
				OperationName: "Seed import approval",
				RiskLevel:     "red",
				TargetObject:  "workspace/reports/seed-report.md",
				Reason:        "Seeded approval",
				Status:        "pending",
				CreatedAt:     "2026-04-19T08:25:00Z",
				UpdatedAt:     "2026-04-19T08:25:00Z",
			}},
			AuthorizationRecords: []bootstrapSeedAuthorizationRecord{{
				AuthorizationRecordID: "auth_seed_001",
				TaskID:                "task_seed_completed",
				ApprovalID:            "approval_seed_archived",
				Decision:              "allow_once",
				Operator:              "seed_test",
				RememberRule:          false,
				CreatedAt:             "2026-04-18T09:10:00Z",
			}},
			AuditRecords: []bootstrapSeedAuditRecord{{
				AuditID:   "audit_seed_001",
				TaskID:    "task_seed_completed",
				Type:      "file",
				Action:    "write_file",
				Summary:   "Seeded audit record",
				Target:    "workspace/reports/seed-report.md",
				Result:    "success",
				CreatedAt: "2026-04-18T09:20:00Z",
			}},
			RecoveryPoints: []bootstrapSeedRecoveryPoint{{
				RecoveryPointID: "rp_seed_001",
				TaskID:          "task_seed_completed",
				Summary:         "Seed recovery point",
				CreatedAt:       "2026-04-18T09:05:00Z",
				Objects:         []string{"workspace/reports/seed-report.md"},
			}},
		},
		"settings.json": bootstrapSeedSettingsState{
			Settings: map[string]any{
				"general": map[string]any{
					"language": "zh-CN",
					"download": map[string]any{
						"workspace_path": "workspace",
					},
				},
				"task_automation": map[string]any{
					"task_sources": []string{"workspace/todos"},
				},
			},
		},
	}

	for relativePath, value := range files {
		if strings.HasSuffix(relativePath, ".json") {
			if err := writeJSONFile(filepath.Join(seedDir, relativePath), value); err != nil {
				return err
			}
		}
	}
	if err := os.WriteFile(filepath.Join(seedDir, "workspace", "reports", "seed-report.md"), []byte("# Seed report\n"), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(seedDir, "workspace", "todos", "inbox.md"), []byte("- [ ] Seeded note\n"), 0o644); err != nil {
		return err
	}
	return nil
}

func writeJSONFile(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}
