package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestInMemoryConfigAssetStoresPersistRecords(t *testing.T) {
	skillStore := newInMemorySkillManifestStore()
	blueprintStore := newInMemoryBlueprintDefinitionStore()
	promptStore := newInMemoryPromptTemplateVersionStore()

	if err := skillStore.WriteSkillManifest(context.Background(), SkillManifestRecord{SkillManifestID: "skill_001", Name: "read_only_skill", Version: "v1", Source: "builtin", Summary: "summary", ManifestJSON: `{}`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write skill manifest failed: %v", err)
	}
	if err := blueprintStore.WriteBlueprintDefinition(context.Background(), BlueprintDefinitionRecord{BlueprintDefinitionID: "blueprint_001", Name: "document_blueprint", Version: "v1", Source: "builtin", Summary: "summary", DefinitionJSON: `{}`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write blueprint definition failed: %v", err)
	}
	if err := promptStore.WritePromptTemplateVersion(context.Background(), PromptTemplateVersionRecord{PromptTemplateVersionID: "prompt_001", TemplateName: "default", Version: "v1", Source: "builtin", Summary: "summary", TemplateBody: "body", VariablesJSON: `[]`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write prompt template version failed: %v", err)
	}
	skillRecord, err := skillStore.GetSkillManifest(context.Background(), "skill_001")
	if err != nil || skillRecord.Name != "read_only_skill" {
		t.Fatalf("unexpected skill manifest lookup: record=%+v err=%v", skillRecord, err)
	}
	blueprintRecord, err := blueprintStore.GetBlueprintDefinition(context.Background(), "blueprint_001")
	if err != nil || blueprintRecord.Name != "document_blueprint" {
		t.Fatalf("unexpected blueprint lookup: record=%+v err=%v", blueprintRecord, err)
	}
	promptRecord, err := promptStore.GetPromptTemplateVersion(context.Background(), "prompt_001")
	if err != nil || promptRecord.TemplateName != "default" {
		t.Fatalf("unexpected prompt template lookup: record=%+v err=%v", promptRecord, err)
	}
	skillItems, skillTotal, err := skillStore.ListSkillManifests(context.Background(), 0, 0)
	if err != nil || skillTotal != 1 || len(skillItems) != 1 {
		t.Fatalf("unexpected skill manifest listing: total=%d items=%+v err=%v", skillTotal, skillItems, err)
	}
	blueprintItems, blueprintTotal, err := blueprintStore.ListBlueprintDefinitions(context.Background(), 0, 0)
	if err != nil || blueprintTotal != 1 || len(blueprintItems) != 1 {
		t.Fatalf("unexpected blueprint listing: total=%d items=%+v err=%v", blueprintTotal, blueprintItems, err)
	}
	promptItems, promptTotal, err := promptStore.ListPromptTemplateVersions(context.Background(), 0, 0)
	if err != nil || promptTotal != 1 || len(promptItems) != 1 {
		t.Fatalf("unexpected prompt template listing: total=%d items=%+v err=%v", promptTotal, promptItems, err)
	}
	if _, err := skillStore.GetSkillManifest(context.Background(), "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected missing in-memory skill manifest to return sql.ErrNoRows, got %v", err)
	}
	if _, err := blueprintStore.GetBlueprintDefinition(context.Background(), "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected missing in-memory blueprint definition to return sql.ErrNoRows, got %v", err)
	}
	if _, err := promptStore.GetPromptTemplateVersion(context.Background(), "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected missing in-memory prompt template version to return sql.ErrNoRows, got %v", err)
	}
}

func TestSQLiteConfigAssetStoresPersistRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config-assets.db")
	skillStore, err := NewSQLiteSkillManifestStore(path)
	if err != nil {
		t.Fatalf("new sqlite skill manifest store failed: %v", err)
	}
	defer func() { _ = skillStore.Close() }()
	blueprintStore, err := NewSQLiteBlueprintDefinitionStore(path)
	if err != nil {
		t.Fatalf("new sqlite blueprint store failed: %v", err)
	}
	defer func() { _ = blueprintStore.Close() }()
	promptStore, err := NewSQLitePromptTemplateVersionStore(path)
	if err != nil {
		t.Fatalf("new sqlite prompt template store failed: %v", err)
	}
	defer func() { _ = promptStore.Close() }()

	if err := skillStore.WriteSkillManifest(context.Background(), SkillManifestRecord{SkillManifestID: "skill_001", Name: "read_only_skill", Version: "v1", Source: "builtin", Summary: "summary", ManifestJSON: `{}`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write skill manifest failed: %v", err)
	}
	if err := blueprintStore.WriteBlueprintDefinition(context.Background(), BlueprintDefinitionRecord{BlueprintDefinitionID: "blueprint_001", Name: "document_blueprint", Version: "v1", Source: "builtin", Summary: "summary", DefinitionJSON: `{}`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write blueprint definition failed: %v", err)
	}
	if err := promptStore.WritePromptTemplateVersion(context.Background(), PromptTemplateVersionRecord{PromptTemplateVersionID: "prompt_001", TemplateName: "default", Version: "v1", Source: "builtin", Summary: "summary", TemplateBody: "body", VariablesJSON: `[]`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write prompt template version failed: %v", err)
	}
	skillRecord, err := skillStore.GetSkillManifest(context.Background(), "skill_001")
	if err != nil || skillRecord.Name != "read_only_skill" {
		t.Fatalf("unexpected sqlite skill manifest lookup: record=%+v err=%v", skillRecord, err)
	}
	skillItems, skillTotal, err := skillStore.ListSkillManifests(context.Background(), 0, 0)
	if err != nil || skillTotal != 1 || len(skillItems) != 1 {
		t.Fatalf("unexpected sqlite skill listing: total=%d items=%+v err=%v", skillTotal, skillItems, err)
	}
	blueprintItems, blueprintTotal, err := blueprintStore.ListBlueprintDefinitions(context.Background(), 0, 0)
	if err != nil || blueprintTotal != 1 || len(blueprintItems) != 1 {
		t.Fatalf("unexpected sqlite blueprint listing: total=%d items=%+v err=%v", blueprintTotal, blueprintItems, err)
	}
	if _, err := blueprintStore.GetBlueprintDefinition(context.Background(), "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected missing blueprint to return sql.ErrNoRows, got %v", err)
	}
	promptItems, promptTotal, err := promptStore.ListPromptTemplateVersions(context.Background(), 0, 0)
	if err != nil || promptTotal != 1 || len(promptItems) != 1 {
		t.Fatalf("unexpected sqlite prompt listing: total=%d items=%+v err=%v", promptTotal, promptItems, err)
	}
	assertSQLiteConfigAssetPragmas(t, skillStore.db)
	assertSQLiteConfigAssetPragmas(t, blueprintStore.db)
	assertSQLiteConfigAssetPragmas(t, promptStore.db)
}

func TestConfigureConfigAssetSQLiteDatabaseSetsBusyTimeoutAndWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config-asset-pragmas.db")
	db, err := openSQLiteDatabase(path)
	if err != nil {
		t.Fatalf("open sqlite database failed: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := configureConfigAssetSQLiteDatabase(context.Background(), db); err != nil {
		t.Fatalf("configure sqlite config asset pragmas failed: %v", err)
	}
	assertSQLiteConfigAssetPragmas(t, db)
}

func TestSQLiteConfigAssetStoresHandleConstructorAndCloseEdgeCases(t *testing.T) {
	if _, err := NewSQLiteSkillManifestStore("   "); err == nil {
		t.Fatal("expected sqlite skill manifest constructor to reject empty path")
	}
	if _, err := NewSQLiteBlueprintDefinitionStore("   "); err == nil {
		t.Fatal("expected sqlite blueprint constructor to reject empty path")
	}
	if _, err := NewSQLitePromptTemplateVersionStore("   "); err == nil {
		t.Fatal("expected sqlite prompt constructor to reject empty path")
	}
	var nilSkillStore SQLiteSkillManifestStore
	if err := nilSkillStore.Close(); err != nil {
		t.Fatalf("expected nil sqlite skill store close to succeed, got %v", err)
	}
	var nilBlueprintStore SQLiteBlueprintDefinitionStore
	if err := nilBlueprintStore.Close(); err != nil {
		t.Fatalf("expected nil sqlite blueprint store close to succeed, got %v", err)
	}
	var nilPromptStore SQLitePromptTemplateVersionStore
	if err := nilPromptStore.Close(); err != nil {
		t.Fatalf("expected nil sqlite prompt store close to succeed, got %v", err)
	}
}

func TestConfigAssetPaginationHelpersCoverEdgeCases(t *testing.T) {
	skillItems := []SkillManifestRecord{{SkillManifestID: "skill_001"}, {SkillManifestID: "skill_002"}}
	if paged := pageSkillManifests(skillItems, 1, 1); len(paged) != 1 || paged[0].SkillManifestID != "skill_002" {
		t.Fatalf("unexpected skill manifest page: %+v", paged)
	}
	if paged := pageSkillManifests(skillItems, 0, -1); len(paged) != 2 {
		t.Fatalf("expected unlimited skill manifest page, got %+v", paged)
	}
	if paged := pageBlueprintDefinitions([]BlueprintDefinitionRecord{{BlueprintDefinitionID: "blueprint_001"}}, 1, 9); paged != nil {
		t.Fatalf("expected nil blueprint page when offset exceeds length, got %+v", paged)
	}
	if paged := pagePromptTemplateVersions([]PromptTemplateVersionRecord{{PromptTemplateVersionID: "prompt_001"}, {PromptTemplateVersionID: "prompt_002"}}, 0, 1); len(paged) != 1 || paged[0].PromptTemplateVersionID != "prompt_002" {
		t.Fatalf("unexpected prompt template page: %+v", paged)
	}
}

func assertSQLiteConfigAssetPragmas(t *testing.T, db *sql.DB) {
	t.Helper()
	var journalMode string
	if err := db.QueryRow(`PRAGMA journal_mode;`).Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode pragma failed: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("expected journal_mode=wal, got %s", journalMode)
	}
	var busyTimeout int
	if err := db.QueryRow(`PRAGMA busy_timeout;`).Scan(&busyTimeout); err != nil {
		t.Fatalf("read busy_timeout pragma failed: %v", err)
	}
	if busyTimeout != 5000 {
		t.Fatalf("expected busy_timeout=5000, got %d", busyTimeout)
	}
}
