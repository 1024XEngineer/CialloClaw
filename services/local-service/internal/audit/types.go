package audit

import "context"

// RecordInput 是 audit 模块当前最小输入结构。
//
// 字段语义对齐协议中的 AuditRecord，
// 但本类型仅用于后端模块内部，不替代协议真源。
type RecordInput struct {
	TaskID  string
	Type    string
	Action  string
	Summary string
	Target  string
	Result  string
}

// Record 是 audit 模块当前最小输出结构。
//
// created_at 使用 RFC3339 时间字符串，便于后续持久化与协议映射。
type Record struct {
	AuditID   string
	TaskID    string
	Type      string
	Action    string
	Summary   string
	Target    string
	Result    string
	CreatedAt string
}

// Writer 是审计记录输出边界。
//
// 当前不直接绑定数据库实现，后续由 storage 或其他持久化层注入。
type Writer interface {
	WriteAuditRecord(ctx context.Context, record Record) error
}
