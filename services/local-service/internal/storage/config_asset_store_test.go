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
	pluginStore := newInMemoryPluginManifestStore()

	if err := skillStore.WriteSkillManifest(context.Background(), SkillManifestRecord{SkillManifestID: "skill_001", Name: "read_only_skill", Version: "v1", Source: "builtin", Summary: "summary", ManifestJSON: `{}`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write skill manifest failed: %v", err)
	}
	if err := blueprintStore.WriteBlueprintDefinition(context.Background(), BlueprintDefinitionRecord{BlueprintDefinitionID: "blueprint_001", Name: "document_blueprint", Version: "v1", Source: "builtin", Summary: "summary", DefinitionJSON: `{}`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write blueprint definition failed: %v", err)
	}
	if err := promptStore.WritePromptTemplateVersion(context.Background(), PromptTemplateVersionRecord{PromptTemplateVersionID: "prompt_001", TemplateName: "default", Version: "v1", Source: "builtin", Summary: "summary", TemplateBody: "body", VariablesJSON: `[]`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write prompt template version failed: %v", err)
	}
	if err := pluginStore.WritePluginManifest(context.Background(), PluginManifestRecord{PluginID: "plugin_001", Name: "ocr", Version: "v1", Entry: "builtin://plugin/ocr", Source: "builtin", Summary: "summary", CapabilitiesJSON: `["ocr_image"]`, PermissionsJSON: `["artifact_read"]`, RuntimeNamesJSON: `["ocr_worker"]`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write plugin manifest failed: %v", err)
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
	pluginRecord, err := pluginStore.GetPluginManifest(context.Background(), "plugin_001")
	if err != nil || pluginRecord.Name != "ocr" {
		t.Fatalf("unexpected plugin manifest lookup: record=%+v err=%v", pluginRecord, err)
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
	pluginItems, pluginTotal, err := pluginStore.ListPluginManifests(context.Background(), 0, 0)
	if err != nil || pluginTotal != 1 || len(pluginItems) != 1 {
		t.Fatalf("unexpected plugin manifest listing: total=%d items=%+v err=%v", pluginTotal, pluginItems, err)
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
	if _, err := pluginStore.GetPluginManifest(context.Background(), "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected missing in-memory plugin manifest to return sql.ErrNoRows, got %v", err)
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
	pluginStore, err := NewSQLitePluginManifestStore(path)
	if err != nil {
		t.Fatalf("new sqlite plugin manifest store failed: %v", err)
	}
	defer func() { _ = pluginStore.Close() }()

	if err := skillStore.WriteSkillManifest(context.Background(), SkillManifestRecord{SkillManifestID: "skill_001", Name: "read_only_skill", Version: "v1", Source: "builtin", Summary: "summary", ManifestJSON: `{}`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write skill manifest failed: %v", err)
	}
	if err := blueprintStore.WriteBlueprintDefinition(context.Background(), BlueprintDefinitionRecord{BlueprintDefinitionID: "blueprint_001", Name: "document_blueprint", Version: "v1", Source: "builtin", Summary: "summary", DefinitionJSON: `{}`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write blueprint definition failed: %v", err)
	}
	if err := promptStore.WritePromptTemplateVersion(context.Background(), PromptTemplateVersionRecord{PromptTemplateVersionID: "prompt_001", TemplateName: "default", Version: "v1", Source: "builtin", Summary: "summary", TemplateBody: "body", VariablesJSON: `[]`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write prompt template version failed: %v", err)
	}
	if err := pluginStore.WritePluginManifest(context.Background(), PluginManifestRecord{PluginID: "plugin_001", Name: "ocr", Version: "v1", Entry: "builtin://plugin/ocr", Source: "builtin", Summary: "summary", CapabilitiesJSON: `["ocr_image"]`, PermissionsJSON: `["artifact_read"]`, RuntimeNamesJSON: `["ocr_worker"]`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write plugin manifest failed: %v", err)
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
	pluginItems, pluginTotal, err := pluginStore.ListPluginManifests(context.Background(), 0, 0)
	if err != nil || pluginTotal != 1 || len(pluginItems) != 1 || pluginItems[0].PluginID != "plugin_001" {
		t.Fatalf("unexpected sqlite plugin listing: total=%d items=%+v err=%v", pluginTotal, pluginItems, err)
	}
	pluginRecord, err := pluginStore.GetPluginManifest(context.Background(), "plugin_001")
	if err != nil || pluginRecord.Name != "ocr" {
		t.Fatalf("unexpected sqlite plugin manifest lookup: record=%+v err=%v", pluginRecord, err)
	}
	assertSQLiteConfigAssetPragmas(t, skillStore.db)
	assertSQLiteConfigAssetPragmas(t, blueprintStore.db)
	assertSQLiteConfigAssetPragmas(t, promptStore.db)
	assertSQLiteConfigAssetPragmas(t, pluginStore.db)
}

func TestInMemoryConfigAssetSourceLookupsPreferLatestMatchingSource(t *testing.T) {
	skillStore := newInMemorySkillManifestStore()
	if err := skillStore.WriteSkillManifest(context.Background(), SkillManifestRecord{SkillManifestID: "skill_builtin_old", Name: "builtin_old", Version: "v1", Source: "builtin", Summary: "summary", ManifestJSON: `{}`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write old builtin skill manifest failed: %v", err)
	}
	if err := skillStore.WriteSkillManifest(context.Background(), SkillManifestRecord{SkillManifestID: "skill_community_latest", Name: "community", Version: "v2", Source: "github", Summary: "summary", ManifestJSON: `{}`, CreatedAt: "2026-04-19T12:00:00Z", UpdatedAt: "2026-04-19T12:00:00Z"}); err != nil {
		t.Fatalf("write latest community skill manifest failed: %v", err)
	}
	if err := skillStore.WriteSkillManifest(context.Background(), SkillManifestRecord{SkillManifestID: "skill_builtin_new", Name: "builtin_new", Version: "v3", Source: "builtin", Summary: "summary", ManifestJSON: `{}`, CreatedAt: "2026-04-19T11:00:00Z", UpdatedAt: "2026-04-19T11:00:00Z"}); err != nil {
		t.Fatalf("write newest builtin skill manifest failed: %v", err)
	}
	record, found, err := skillStore.latestSkillManifestBySource(context.Background(), " builtin ")
	if err != nil || !found || record.SkillManifestID != "skill_builtin_new" {
		t.Fatalf("expected latest builtin skill manifest, record=%+v found=%v err=%v", record, found, err)
	}

	blueprintStore := newInMemoryBlueprintDefinitionStore()
	if err := blueprintStore.WriteBlueprintDefinition(context.Background(), BlueprintDefinitionRecord{BlueprintDefinitionID: "blueprint_community", Name: "community", Version: "v1", Source: "github", Summary: "summary", DefinitionJSON: `{}`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write community blueprint failed: %v", err)
	}
	if _, found, err := blueprintStore.latestBlueprintDefinitionBySource(context.Background(), "builtin"); err != nil || found {
		t.Fatalf("expected no builtin blueprint match, found=%v err=%v", found, err)
	}

	promptStore := newInMemoryPromptTemplateVersionStore()
	if err := promptStore.WritePromptTemplateVersion(context.Background(), PromptTemplateVersionRecord{PromptTemplateVersionID: "prompt_builtin_a", TemplateName: "default", Version: "v1", Source: "builtin", Summary: "summary", TemplateBody: "body", VariablesJSON: `[]`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write builtin prompt v1 failed: %v", err)
	}
	if err := promptStore.WritePromptTemplateVersion(context.Background(), PromptTemplateVersionRecord{PromptTemplateVersionID: "prompt_builtin_b", TemplateName: "default", Version: "v2", Source: "builtin", Summary: "summary", TemplateBody: "body", VariablesJSON: `[]`, CreatedAt: "2026-04-19T11:00:00Z", UpdatedAt: "2026-04-19T11:00:00Z"}); err != nil {
		t.Fatalf("write builtin prompt v2 failed: %v", err)
	}
	recordPrompt, found, err := promptStore.latestPromptTemplateVersionBySource(context.Background(), "builtin")
	if err != nil || !found || recordPrompt.PromptTemplateVersionID != "prompt_builtin_b" {
		t.Fatalf("expected newest builtin prompt template version, record=%+v found=%v err=%v", recordPrompt, found, err)
	}
}

func TestSQLiteConfigAssetSourceLookupsPreferLatestMatchingSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config-assets-latest.db")
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
		t.Fatalf("new sqlite prompt store failed: %v", err)
	}
	defer func() { _ = promptStore.Close() }()

	if err := skillStore.WriteSkillManifest(context.Background(), SkillManifestRecord{SkillManifestID: "skill_builtin_old", Name: "builtin_old", Version: "v1", Source: "builtin", Summary: "summary", ManifestJSON: `{}`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write builtin skill manifest failed: %v", err)
	}
	if err := skillStore.WriteSkillManifest(context.Background(), SkillManifestRecord{SkillManifestID: "skill_community_latest", Name: "community", Version: "v2", Source: "github", Summary: "summary", ManifestJSON: `{}`, CreatedAt: "2026-04-19T12:00:00Z", UpdatedAt: "2026-04-19T12:00:00Z"}); err != nil {
		t.Fatalf("write community skill manifest failed: %v", err)
	}
	if err := skillStore.WriteSkillManifest(context.Background(), SkillManifestRecord{SkillManifestID: "skill_builtin_new", Name: "builtin_new", Version: "v3", Source: "builtin", Summary: "summary", ManifestJSON: `{}`, CreatedAt: "2026-04-19T11:00:00Z", UpdatedAt: "2026-04-19T11:00:00Z"}); err != nil {
		t.Fatalf("write second builtin skill manifest failed: %v", err)
	}
	record, found, err := skillStore.latestSkillManifestBySource(context.Background(), "builtin")
	if err != nil || !found || record.SkillManifestID != "skill_builtin_new" {
		t.Fatalf("expected latest builtin sqlite skill manifest, record=%+v found=%v err=%v", record, found, err)
	}
	if _, found, err := skillStore.latestSkillManifestBySource(context.Background(), "marketplace"); err != nil || found {
		t.Fatalf("expected no sqlite skill manifest for marketplace source, found=%v err=%v", found, err)
	}

	if err := blueprintStore.WriteBlueprintDefinition(context.Background(), BlueprintDefinitionRecord{BlueprintDefinitionID: "blueprint_builtin", Name: "builtin", Version: "v1", Source: "builtin", Summary: "summary", DefinitionJSON: `{}`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write builtin blueprint failed: %v", err)
	}
	recordBlueprint, found, err := blueprintStore.latestBlueprintDefinitionBySource(context.Background(), "builtin")
	if err != nil || !found || recordBlueprint.BlueprintDefinitionID != "blueprint_builtin" {
		t.Fatalf("expected builtin sqlite blueprint match, record=%+v found=%v err=%v", recordBlueprint, found, err)
	}
	if err := blueprintStore.Close(); err != nil {
		t.Fatalf("close sqlite blueprint store failed: %v", err)
	}
	if _, found, err := blueprintStore.latestBlueprintDefinitionBySource(context.Background(), "builtin"); err == nil || found {
		t.Fatalf("expected closed sqlite blueprint store to return error, found=%v err=%v", found, err)
	}

	if _, found, err := promptStore.latestPromptTemplateVersionBySource(context.Background(), "builtin"); err != nil || found {
		t.Fatalf("expected no builtin sqlite prompt match before inserts, found=%v err=%v", found, err)
	}
	if err := promptStore.WritePromptTemplateVersion(context.Background(), PromptTemplateVersionRecord{PromptTemplateVersionID: "prompt_builtin", TemplateName: "default", Version: "v1", Source: "builtin", Summary: "summary", TemplateBody: "body", VariablesJSON: `[]`, CreatedAt: "2026-04-19T10:00:00Z", UpdatedAt: "2026-04-19T10:00:00Z"}); err != nil {
		t.Fatalf("write builtin prompt failed: %v", err)
	}
	recordPrompt, found, err := promptStore.latestPromptTemplateVersionBySource(context.Background(), "builtin")
	if err != nil || !found || recordPrompt.PromptTemplateVersionID != "prompt_builtin" {
		t.Fatalf("expected builtin sqlite prompt match, record=%+v found=%v err=%v", recordPrompt, found, err)
	}
}

func TestConfigAssetMoreRecentCoversOrderingBranches(t *testing.T) {
	if !configAssetMoreRecent("2026-04-19T11:00:00Z", "b", "2026-04-19T10:00:00Z", "a") {
		t.Fatal("expected newer timestamp to win")
	}
	if !configAssetMoreRecent("2026-04-19T11:00:00Z", "b", "2026-04-19T11:00:00Z", "a") {
		t.Fatal("expected lexicographically larger id to win ties")
	}
	if configAssetMoreRecent("2026-04-19T11:00:00Z", "a", "2026-04-19T11:00:00Z", "b") {
		t.Fatal("expected smaller id to lose timestamp ties")
	}
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
	if _, err := NewSQLitePluginManifestStore("   "); err == nil {
		t.Fatal("expected sqlite plugin manifest constructor to reject empty path")
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
	var nilPluginStore SQLitePluginManifestStore
	if err := nilPluginStore.Close(); err != nil {
		t.Fatalf("expected nil sqlite plugin manifest store close to succeed, got %v", err)
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
	if busyTimeout != 300000 {
		t.Fatalf("expected busy_timeout=300000, got %d", busyTimeout)
	}
}
