package audit

import "context"

// RecordInput is the minimal input shape accepted by the audit module.
//
// Its field semantics mirror the protocol-level AuditRecord, but this type is
// only used inside backend services and does not replace the protocol source of
// truth.
type RecordInput struct {
	TaskID  string
	RunID   string
	Type    string
	Action  string
	Summary string
	Target  string
	Result  string
}

// Record is the minimal output shape produced by the audit module.
//
// created_at uses an RFC3339 timestamp string so storage and protocol mapping
// can reuse the same value without extra normalization.
type Record struct {
	AuditID   string
	TaskID    string
	RunID     string
	Type      string
	Action    string
	Summary   string
	Target    string
	Result    string
	CreatedAt string
}

// Map converts the minimal audit record into a structured map for callers.
func (r Record) Map() map[string]any {
	return map[string]any{
		"audit_id":   r.AuditID,
		"task_id":    r.TaskID,
		"run_id":     r.RunID,
		"type":       r.Type,
		"action":     r.Action,
		"summary":    r.Summary,
		"target":     r.Target,
		"result":     r.Result,
		"created_at": r.CreatedAt,
	}
}

// Writer is the persistence boundary for audit records.
//
// The audit module stays storage-agnostic here so storage or another
// persistence adapter can be injected by the caller.
type Writer interface {
	WriteAuditRecord(ctx context.Context, record Record) error
}
