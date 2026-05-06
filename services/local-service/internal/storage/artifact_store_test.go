package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestInMemoryArtifactStoreReplacesDuplicateArtifactIDs(t *testing.T) {
	store := newInMemoryArtifactStore()
	err := store.SaveArtifacts(context.Background(), []ArtifactRecord{{
		ArtifactID:          "art_001",
		TaskID:              "task_001",
		RunID:               "run_001",
		ArtifactType:        "generated_doc",
		Title:               "first.md",
		Path:                "workspace/first.md",
		MimeType:            "text/markdown",
		DeliveryType:        "workspace_document",
		DeliveryPayloadJSON: `{"path":"workspace/first.md","task_id":"task_001"}`,
		CreatedAt:           "2026-04-14T10:00:00Z",
	}})
	if err != nil {
		t.Fatalf("initial save failed: %v", err)
	}
	err = store.SaveArtifacts(context.Background(), []ArtifactRecord{{
		ArtifactID:          "art_001",
		TaskID:              "task_001",
		RunID:               "run_001",
		ArtifactType:        "generated_doc",
		Title:               "updated.md",
		Path:                "workspace/updated.md",
		MimeType:            "text/markdown",
		DeliveryType:        "workspace_document",
		DeliveryPayloadJSON: `{"path":"workspace/updated.md","task_id":"task_001"}`,
		CreatedAt:           "2026-04-14T10:01:00Z",
	}})
	if err != nil {
		t.Fatalf("replacement save failed: %v", err)
	}
	items, total, err := store.ListArtifacts(context.Background(), "task_001", "", 20, 0)
	if err != nil {
		t.Fatalf("list artifacts failed: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected one replaced artifact, got total=%d items=%+v", total, items)
	}
	if items[0].Title != "updated.md" || items[0].Path != "workspace/updated.md" {
		t.Fatalf("expected replacement artifact payload, got %+v", items[0])
	}
	if items[0].RunID != "run_001" {
		t.Fatalf("expected artifact run_id to round-trip, got %+v", items[0])
	}
}

func TestValidateArtifactRecordRejectsInvalidPayloadJSON(t *testing.T) {
	err := validateArtifactRecord(ArtifactRecord{
		ArtifactID:          "art_invalid",
		TaskID:              "task_001",
		ArtifactType:        "generated_doc",
		Title:               "invalid.md",
		Path:                "workspace/invalid.md",
		MimeType:            "text/markdown",
		DeliveryType:        "workspace_document",
		DeliveryPayloadJSON: `{"path":`,
		CreatedAt:           "2026-04-14T10:00:00Z",
	})
	if err == nil {
		t.Fatal("expected invalid payload json to be rejected")
	}
}

func TestValidateArtifactRecordAcceptsRFC3339Nano(t *testing.T) {
	err := validateArtifactRecord(ArtifactRecord{
		ArtifactID:          "art_valid",
		TaskID:              "task_001",
		ArtifactType:        "generated_doc",
		Title:               "valid.md",
		Path:                "workspace/valid.md",
		MimeType:            "text/markdown",
		DeliveryType:        "workspace_document",
		DeliveryPayloadJSON: `{"path":"workspace/valid.md","task_id":"task_001"}`,
		CreatedAt:           "2026-04-14T10:00:00.123456789Z",
	})
	if err != nil {
		t.Fatalf("expected rfc3339nano timestamp to be accepted, got %v", err)
	}
}

func TestInMemoryArtifactStoreListArtifactsAppliesPaging(t *testing.T) {
	store := newInMemoryArtifactStore()
	err := store.SaveArtifacts(context.Background(), []ArtifactRecord{
		{
			ArtifactID:          "art_001",
			TaskID:              "task_001",
			ArtifactType:        "generated_doc",
			Title:               "one.md",
			Path:                "workspace/one.md",
			MimeType:            "text/markdown",
			DeliveryType:        "workspace_document",
			DeliveryPayloadJSON: `{"path":"workspace/one.md","task_id":"task_001"}`,
			CreatedAt:           "2026-04-14T10:00:00Z",
		},
		{
			ArtifactID:          "art_002",
			TaskID:              "task_001",
			ArtifactType:        "generated_doc",
			Title:               "two.md",
			Path:                "workspace/two.md",
			MimeType:            "text/markdown",
			DeliveryType:        "workspace_document",
			DeliveryPayloadJSON: `{"path":"workspace/two.md","task_id":"task_001"}`,
			CreatedAt:           "2026-04-14T10:01:00Z",
		},
		{
			ArtifactID:          "art_003",
			TaskID:              "task_002",
			ArtifactType:        "generated_doc",
			Title:               "three.md",
			Path:                "workspace/three.md",
			MimeType:            "text/markdown",
			DeliveryType:        "workspace_document",
			DeliveryPayloadJSON: `{"path":"workspace/three.md","task_id":"task_002"}`,
			CreatedAt:           "2026-04-14T10:02:00Z",
		},
	})
	if err != nil {
		t.Fatalf("save artifacts failed: %v", err)
	}
	items, total, err := store.ListArtifacts(context.Background(), "task_001", "", 1, 1)
	if err != nil {
		t.Fatalf("list artifacts failed: %v", err)
	}
	if total != 2 || len(items) != 1 {
		t.Fatalf("expected paged result for task_001, got total=%d items=%+v", total, items)
	}
	if items[0].ArtifactID != "art_001" {
		t.Fatalf("expected second newest artifact for task_001, got %+v", items[0])
	}
	items, total, err = store.ListArtifacts(context.Background(), "", "", -1, -5)
	if err != nil {
		t.Fatalf("list all artifacts failed: %v", err)
	}
	if total != 3 || len(items) != 3 {
		t.Fatalf("expected negative paging values to normalize, got total=%d items=%+v", total, items)
	}
}

func TestSQLiteArtifactStorePersistsReplacesAndPages(t *testing.T) {
	store, err := NewSQLiteArtifactStore(filepath.Join(t.TempDir(), "artifacts.db"))
	if err != nil {
		t.Fatalf("new sqlite artifact store failed: %v", err)
	}
	defer func() { _ = store.Close() }()
	records := []ArtifactRecord{
		{
			ArtifactID:          "art_sql_001",
			TaskID:              "task_sql",
			RunID:               "run_sql",
			ArtifactType:        "generated_doc",
			Title:               "one.md",
			Path:                "workspace/one.md",
			MimeType:            "text/markdown",
			DeliveryType:        "workspace_document",
			DeliveryPayloadJSON: `{"path":"workspace/one.md","task_id":"task_sql"}`,
			CreatedAt:           "2026-04-14T10:00:00Z",
		},
		{
			ArtifactID:          "art_sql_002",
			TaskID:              "task_sql",
			ArtifactType:        "generated_doc",
			Title:               "two.md",
			Path:                "workspace/two.md",
			MimeType:            "text/markdown",
			DeliveryType:        "workspace_document",
			DeliveryPayloadJSON: `{"path":"workspace/two.md","task_id":"task_sql"}`,
			CreatedAt:           "2026-04-14T10:01:00Z",
		},
	}
	if err := store.SaveArtifacts(context.Background(), records); err != nil {
		t.Fatalf("save sqlite artifacts failed: %v", err)
	}
	if err := store.SaveArtifacts(context.Background(), []ArtifactRecord{{
		ArtifactID:          "art_sql_001",
		TaskID:              "task_sql",
		RunID:               "run_sql",
		ArtifactType:        "generated_doc",
		Title:               "one-updated.md",
		Path:                "workspace/one-updated.md",
		MimeType:            "text/markdown",
		DeliveryType:        "open_file",
		DeliveryPayloadJSON: `{"path":"workspace/one-updated.md","task_id":"task_sql"}`,
		CreatedAt:           "2026-04-14T10:02:00Z",
	}}); err != nil {
		t.Fatalf("replace sqlite artifact failed: %v", err)
	}
	items, total, err := store.ListArtifacts(context.Background(), "task_sql", "", 1, 0)
	if err != nil {
		t.Fatalf("list sqlite artifacts failed: %v", err)
	}
	if total != 2 || len(items) != 1 {
		t.Fatalf("expected paged sqlite artifacts, got total=%d items=%+v", total, items)
	}
	if items[0].ArtifactID != "art_sql_001" || items[0].RunID != "run_sql" || items[0].Title != "one-updated.md" || items[0].DeliveryType != "open_file" {
		t.Fatalf("expected replacement artifact to sort first, got %+v", items[0])
	}
	items, total, err = store.ListArtifacts(context.Background(), "task_sql", "", 0, 0)
	if err != nil {
		t.Fatalf("list all sqlite artifacts failed: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("expected full sqlite artifact list, got total=%d items=%+v", total, items)
	}
	items, total, err = store.ListArtifacts(context.Background(), "task_sql", "run_sql", 0, 0)
	if err != nil {
		t.Fatalf("list sqlite artifacts by run_id failed: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ArtifactID != "art_sql_001" {
		t.Fatalf("expected run-scoped sqlite artifacts, got total=%d items=%+v", total, items)
	}
}

func TestValidateArtifactRecordRejectsMissingRequiredFields(t *testing.T) {
	valid := ArtifactRecord{
		ArtifactID:          "art_valid",
		TaskID:              "task_001",
		ArtifactType:        "generated_doc",
		Title:               "valid.md",
		Path:                "workspace/valid.md",
		MimeType:            "text/markdown",
		DeliveryType:        "workspace_document",
		DeliveryPayloadJSON: `{"path":"workspace/valid.md","task_id":"task_001"}`,
		CreatedAt:           "2026-04-14T10:00:00Z",
	}
	tests := []struct {
		name   string
		mutate func(*ArtifactRecord)
	}{
		{name: "artifact id", mutate: func(record *ArtifactRecord) { record.ArtifactID = "" }},
		{name: "task id", mutate: func(record *ArtifactRecord) { record.TaskID = "" }},
		{name: "artifact type", mutate: func(record *ArtifactRecord) { record.ArtifactType = "" }},
		{name: "title", mutate: func(record *ArtifactRecord) { record.Title = "" }},
		{name: "path", mutate: func(record *ArtifactRecord) { record.Path = "" }},
		{name: "mime type", mutate: func(record *ArtifactRecord) { record.MimeType = "" }},
		{name: "delivery type", mutate: func(record *ArtifactRecord) { record.DeliveryType = "" }},
		{name: "delivery payload", mutate: func(record *ArtifactRecord) { record.DeliveryPayloadJSON = "" }},
		{name: "created at", mutate: func(record *ArtifactRecord) { record.CreatedAt = "" }},
		{name: "bad time", mutate: func(record *ArtifactRecord) { record.CreatedAt = "bad-time" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := valid
			test.mutate(&record)
			if err := validateArtifactRecord(record); err == nil {
				t.Fatalf("expected validation error for %s", test.name)
			}
		})
	}
}

func TestPageArtifactRecordsHandlesOutOfRangeOffset(t *testing.T) {
	items := []ArtifactRecord{{ArtifactID: "art_001"}, {ArtifactID: "art_002"}}
	if paged := pageArtifactRecords(items, 1, 5); paged != nil {
		t.Fatalf("expected nil page for out-of-range offset, got %+v", paged)
	}
}
