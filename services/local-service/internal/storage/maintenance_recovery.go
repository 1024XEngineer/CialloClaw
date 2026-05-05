package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
)

const maintenanceRecoveryRootName = ".maintenance_recovery_points"

type maintenanceRecoveryManifest struct {
	Mode        string                     `json:"mode"`
	GeneratedAt string                     `json:"generated_at"`
	Assets      []maintenanceRecoveryAsset `json:"assets"`
}

type maintenanceRecoveryAsset struct {
	SourcePath string `json:"source_path"`
	BackupPath string `json:"backup_path"`
	Kind       string `json:"kind"`
}

// CreateMaintenanceRecoveryPoint snapshots the backing data files for one
// destructive maintenance action. The resulting recovery_point intentionally
// references manual-restore assets because `agent.security.restore_apply`
// currently restores workspace snapshots only.
func (s *Service) CreateMaintenanceRecoveryPoint(ctx context.Context, taskID, summary string, files []string) (checkpoint.RecoveryPoint, error) {
	checkpointService := checkpoint.NewService()
	normalizedFiles, err := existingMaintenanceRecoveryFiles(files)
	if err != nil {
		return checkpoint.RecoveryPoint{}, err
	}
	normalizedDatabasePath := filepath.Clean(s.DatabasePath())
	normalizedSecretStorePath := filepath.Clean(s.SecretStorePath())
	secretStoreUsesSQLite := maintenanceSecretStoreUsesSQLite(s.secretStore)
	createInput := checkpoint.CreateInput{TaskID: strings.TrimSpace(taskID), Summary: strings.TrimSpace(summary)}
	if len(normalizedFiles) == 0 {
		createInput.Objects = []string{"in_memory_runtime_state"}
		point, err := checkpointService.BuildRecoveryPoint(createInput)
		if err != nil {
			return checkpoint.RecoveryPoint{}, err
		}
		point.Mode = "manual_backup"
		if err := s.RecoveryPointWriter().WriteRecoveryPoint(ctx, point); err != nil {
			return checkpoint.RecoveryPoint{}, fmt.Errorf("write maintenance recovery point: %w", err)
		}
		return point, nil
	}

	createInput.Objects = append([]string(nil), normalizedFiles...)
	point, err := checkpointService.BuildRecoveryPoint(createInput)
	if err != nil {
		return checkpoint.RecoveryPoint{}, err
	}
	point.Mode = "manual_backup"
	backupRoot, err := s.maintenanceRecoveryRoot()
	if err != nil {
		return checkpoint.RecoveryPoint{}, err
	}
	pointDir := filepath.Join(backupRoot, point.RecoveryPointID)
	if err := os.MkdirAll(pointDir, 0o755); err != nil {
		return checkpoint.RecoveryPoint{}, fmt.Errorf("create maintenance recovery directory: %w", err)
	}

	manifest := maintenanceRecoveryManifest{
		Mode:        "manual_restore_only",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Assets:      make([]maintenanceRecoveryAsset, 0, len(normalizedFiles)),
	}
	backupObjects := make([]string, 0, len(normalizedFiles)+1)
	for index, sourcePath := range normalizedFiles {
		backupPath := filepath.Join(pointDir, fmt.Sprintf("%02d_%s", index+1, filepath.Base(sourcePath)))
		preferSQLiteSnapshot := sourcePath == normalizedDatabasePath || (secretStoreUsesSQLite && sourcePath == normalizedSecretStorePath)
		if err := backupMaintenanceSource(ctx, sourcePath, backupPath, preferSQLiteSnapshot); err != nil {
			return checkpoint.RecoveryPoint{}, err
		}
		manifest.Assets = append(manifest.Assets, maintenanceRecoveryAsset{
			SourcePath: filepath.ToSlash(filepath.Clean(sourcePath)),
			BackupPath: filepath.ToSlash(filepath.Clean(backupPath)),
			Kind:       maintenanceRecoveryAssetKind(sourcePath, normalizedDatabasePath, normalizedSecretStorePath),
		})
		backupObjects = append(backupObjects, filepath.ToSlash(filepath.Clean(backupPath)))
	}
	manifestPath := filepath.Join(pointDir, "manifest.json")
	encodedManifest, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return checkpoint.RecoveryPoint{}, fmt.Errorf("encode maintenance recovery manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, encodedManifest, 0o644); err != nil {
		return checkpoint.RecoveryPoint{}, fmt.Errorf("write maintenance recovery manifest: %w", err)
	}
	point.Objects = append([]string{filepath.ToSlash(filepath.Clean(manifestPath))}, backupObjects...)
	if err := s.RecoveryPointWriter().WriteRecoveryPoint(ctx, point); err != nil {
		return checkpoint.RecoveryPoint{}, fmt.Errorf("write maintenance recovery point: %w", err)
	}
	return point, nil
}

func (s *Service) maintenanceRecoveryRoot() (string, error) {
	for _, candidate := range []string{s.DatabasePath(), s.SecretStorePath()} {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			continue
		}
		return filepath.Join(filepath.Dir(trimmed), maintenanceRecoveryRootName), nil
	}
	return "", ErrDatabasePathRequired
}

func existingMaintenanceRecoveryFiles(files []string) ([]string, error) {
	seen := make(map[string]struct{})
	items := make([]string, 0, len(files))
	for _, file := range files {
		trimmed := strings.TrimSpace(file)
		if trimmed == "" {
			continue
		}
		cleaned := filepath.Clean(trimmed)
		if _, exists := seen[cleaned]; exists {
			continue
		}
		if _, err := os.Stat(cleaned); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat maintenance recovery source %s: %w", cleaned, err)
		}
		seen[cleaned] = struct{}{}
		items = append(items, cleaned)
	}
	return items, nil
}

func backupMaintenanceSource(ctx context.Context, sourcePath, backupPath string, preferSQLiteSnapshot bool) error {
	if preferSQLiteSnapshot {
		if err := checkpointSQLiteDatabase(ctx, sourcePath); err != nil {
			return err
		}
	}
	return copyMaintenanceFile(sourcePath, backupPath)
}

func checkpointSQLiteDatabase(ctx context.Context, databasePath string) error {
	db, err := openSQLiteDatabase(databasePath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if _, err := db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE);`); err != nil {
		return fmt.Errorf("checkpoint sqlite database: %w", err)
	}
	return nil
}

func copyMaintenanceFile(sourcePath, backupPath string) error {
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read maintenance source %s: %w", sourcePath, err)
	}
	if err := os.WriteFile(backupPath, content, 0o644); err != nil {
		return fmt.Errorf("write maintenance backup %s: %w", backupPath, err)
	}
	return nil
}

func maintenanceRecoveryAssetKind(sourcePath, databasePath, secretStorePath string) string {
	switch sourcePath {
	case databasePath:
		return "sqlite_database"
	case secretStorePath:
		return "secret_store"
	default:
		return "file"
	}
}

func maintenanceSecretStoreUsesSQLite(secretStore SecretStore) bool {
	_, ok := secretStore.(*SQLiteSecretStore)
	return ok
}
