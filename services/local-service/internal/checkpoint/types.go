package checkpoint

import "context"

// CreateInput 是 checkpoint 模块当前最小输入结构。
//
// 字段语义对齐协议中的 RecoveryPoint，
// 但本类型仅用于后端模块内部，不替代协议真源。
type CreateInput struct {
	TaskID  string
	Summary string
	Objects []string
}

// RecoveryPoint 是 checkpoint 模块当前最小输出结构。
//
// created_at 使用 RFC3339 字符串，便于后续持久化与协议映射。
type RecoveryPoint struct {
	RecoveryPointID string
	TaskID          string
	Summary         string
	CreatedAt       string
	Objects         []string
}

// ApplyResult 描述一次恢复点应用成功后的最小结果。
type ApplyResult struct {
	RecoveryPointID string
	RestoredObjects []string
}

// Writer 是恢复点输出边界。
//
// 当前不绑定数据库实现，后续由 storage 或其他持久化层注入。
type Writer interface {
	WriteRecoveryPoint(ctx context.Context, point RecoveryPoint) error
}

// SnapshotFileSystem 是 checkpoint 对工作区快照与恢复所需的最小文件接口。
type SnapshotFileSystem interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, content []byte) error
	Remove(path string) error
}
