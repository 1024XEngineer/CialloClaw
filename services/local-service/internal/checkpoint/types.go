package checkpoint

import "context"

// CreateInput is the module-local input used to create one recovery point.
// Its fields mirror the formal RecoveryPoint shape without replacing the
// protocol source of truth.
type CreateInput struct {
	TaskID  string
	Summary string
	Objects []string
}

// RecoveryPoint is the module-local recovery point carrier. CreatedAt remains
// an RFC3339 string so storage and protocol mapping can share one timestamp
// representation.
type RecoveryPoint struct {
	RecoveryPointID string
	TaskID          string
	Summary         string
	CreatedAt       string
	Objects         []string
}

// ApplyResult reports the objects restored after a recovery point is applied.
type ApplyResult struct {
	RecoveryPointID string
	RestoredObjects []string
}

// Writer is the recovery-point persistence boundary injected by bootstrap. The
// checkpoint package does not bind itself to a concrete storage implementation.
type Writer interface {
	WriteRecoveryPoint(ctx context.Context, point RecoveryPoint) error
}

// SnapshotFileSystem is the minimal workspace file boundary needed for
// snapshot creation and restoration.
type SnapshotFileSystem interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, content []byte) error
	Remove(path string) error
}
