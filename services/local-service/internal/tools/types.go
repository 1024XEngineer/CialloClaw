// Package tools 定义 CialloClaw 后端工具能力接入层的核心类型与接口。
//
// 本模块只负责 tool registry、tool adapter、tool executor facade、
// builtin tool 和 worker/sidecar client 接入，不负责 intent 识别、
// orchestrator/runengine 状态机、delivery_result 编排或前端协议消费。
//
// 所有 tool 名称使用 snake_case，所有输出结构服从 /packages/protocol，
// 所有工具执行都必须产生 ToolCall 记录，平台相关逻辑必须通过
// platform adapter 注入。
package tools

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
)

// ---------------------------------------------------------------------------
// ToolSource：工具来源分类
// ---------------------------------------------------------------------------

// ToolSource 表示工具的来源类型，用于 registry 和 executor 区分工具能力层级。
type ToolSource string

const (
	// ToolSourceBuiltin 表示本地内置工具，进程内直接执行。
	ToolSourceBuiltin ToolSource = "builtin"
	// ToolSourceWorker 表示通过独立 worker 进程调用的外部工具。
	ToolSourceWorker ToolSource = "worker"
	// ToolSourceSidecar 表示通过 sidecar 进程调用的外部工具。
	ToolSourceSidecar ToolSource = "sidecar"
)

// ---------------------------------------------------------------------------
// ToolMetadata：工具元数据
// ---------------------------------------------------------------------------

// ToolMetadata 描述一个已注册工具的静态元信息。
//
// name 必须使用 snake_case，全局唯一，与 ToolCall.tool_name 对齐。
// display_name 是面向展示的可读名称，不作为注册键。
// description 用于工具发现与推荐场景的简短说明。
// source 标识工具来源：builtin / worker / sidecar。
// risk_hint 提示该工具的风险等级，与统一状态 risk_level 对齐。
// timeout_sec 表示单次执行的超时秒数，0 表示由 executor 默认值接管。
// input_schema_ref 和 output_schema_ref 引用 /packages/protocol 中的
// schema 定义路径，本模块不自行解析 schema，只保留引用。
// supports_dry_run 表示该工具是否支持预检查（dry run）模式。
type ToolMetadata struct {
	Name            string     `json:"name"`
	DisplayName     string     `json:"display_name"`
	Description     string     `json:"description"`
	Source          ToolSource `json:"source"`
	RiskHint        string     `json:"risk_hint"`
	TimeoutSec      int        `json:"timeout_sec"`
	InputSchemaRef  string     `json:"input_schema_ref"`
	OutputSchemaRef string     `json:"output_schema_ref"`
	SupportsDryRun  bool       `json:"supports_dry_run"`
}

// Validate 校验 ToolMetadata 的必填字段与命名规范。
func (m ToolMetadata) Validate() error {
	if m.Name == "" {
		return ErrToolNameRequired
	}
	if !isSnakeCase(m.Name) {
		return fmt.Errorf("%w: %q must be snake_case", ErrToolNameInvalid, m.Name)
	}
	if m.Source == "" {
		return ErrToolSourceRequired
	}
	if m.Source != ToolSourceBuiltin && m.Source != ToolSourceWorker && m.Source != ToolSourceSidecar {
		return fmt.Errorf("%w: %q", ErrToolSourceInvalid, m.Source)
	}
	if m.DisplayName == "" {
		return ErrToolDisplayNameRequired
	}
	return nil
}

// ---------------------------------------------------------------------------
// ToolResult：工具执行结果
// ---------------------------------------------------------------------------

// ToolResult is normalized tool output before executor lifecycle recording.
type ToolResult struct {
	ToolName      string
	RawOutput     map[string]any
	SummaryOutput map[string]any
	Output        map[string]any
	Artifacts     []ArtifactRef
	Error         *ToolResultError
	Duration      time.Duration
}

// ToolExecutionResult is the executor-returned structured result.
type ToolExecutionResult struct {
	Metadata      ToolMetadata
	Precheck      *RiskPrecheckResult
	RawOutput     map[string]any
	SummaryOutput map[string]any
	Artifacts     []ArtifactRef
	Error         *ToolResultError
	Duration      time.Duration
	ToolCall      ToolCallRecord
}

// ToolCallStatus describes the minimal lifecycle state of one tool execution.
type ToolCallStatus string

const (
	ToolCallStatusStarted   ToolCallStatus = "started"
	ToolCallStatusSucceeded ToolCallStatus = "succeeded"
	ToolCallStatusFailed    ToolCallStatus = "failed"
	ToolCallStatusTimeout   ToolCallStatus = "timeout"
)

// ToolCallRecord is the minimal tool_call carrier used by the recorder layer.
// It intentionally keeps only the minimum fields required by the current task.
type ToolCallRecord struct {
	ToolCallID string         `json:"tool_call_id"`
	RunID      string         `json:"run_id"`
	TaskID     string         `json:"task_id"`
	StepID     string         `json:"step_id"`
	ToolName   string         `json:"tool_name"`
	Status     ToolCallStatus `json:"status"`
	Input      map[string]any `json:"input,omitempty"`
	Output     map[string]any `json:"output,omitempty"`
	ErrorCode  *int           `json:"error_code,omitempty"`
	DurationMS int64          `json:"duration_ms"`
}

// ToolCallSink is the storage-agnostic sink interface used by ToolCallRecorder.
// Concrete persistence is injected later and is outside the tools module.
type ToolCallSink interface {
	SaveToolCall(ctx context.Context, record ToolCallRecord) error
}

// ArtifactRef 描述工具执行产生的产物引用。
//
// 本类型不替代 /packages/protocol 中的 Artifact 定义，
// 只在 tools 模块内部用于向上层传递产物信息，
// 上层负责将其映射为正式协议对象。
type ArtifactRef struct {
	ArtifactType string `json:"artifact_type"`
	Title        string `json:"title"`
	Path         string `json:"path"`
	MimeType     string `json:"mime_type"`
}

// ToolResultError 描述工具执行失败的归一化错误信息。
type ToolResultError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// ---------------------------------------------------------------------------
// Tool 接口
// ---------------------------------------------------------------------------

// Tool 是所有工具必须实现的核心接口。
//
// Metadata 返回工具的静态元信息，用于 registry 注册和 executor 查找。
// Validate 在执行前对输入做业务级校验，避免无效输入进入执行路径。
// Execute 执行工具逻辑，接收 ToolExecuteContext 和原始输入，
// 返回归一化的 ToolResult。
//
// 约束：
//   - Execute 必须能产出 ToolCall 记录所需的全部数据；
//   - Execute 不得直接推进 task/run/step/event 状态机；
//   - Execute 不得直接编排 delivery_result；
//   - Execute 不得返回未登记的临时 JSON。
type Tool interface {
	Metadata() ToolMetadata
	Validate(input map[string]any) error
	Execute(ctx context.Context, execCtx *ToolExecuteContext, input map[string]any) (*ToolResult, error)
}

// DryRunTool 是可选接口，支持预检查模式的工具可以实现它。
type DryRunTool interface {
	Tool
	DryRun(ctx context.Context, execCtx *ToolExecuteContext, input map[string]any) (*ToolResult, error)
}

// ---------------------------------------------------------------------------
// ToolExecuteContext：工具执行上下文
// ---------------------------------------------------------------------------

// StorageCapability 是 tools 模块所需的存储能力最小接口。
//
// 不直接引用 storage 包内部类型，避免 tools 与 storage 产生编译期耦合。
// 具体实现由 bootstrap 通过 storage 适配注入。
type StorageCapability interface {
	DatabasePath() string
}

// PlatformCapability 是 tools 模块所需的平台能力最小接口。
//
// 不直接引用 platform 包内部类型，避免 tools 与 platform 产生编译期耦合。
// 具体实现由 bootstrap 通过 platform 适配注入。
type PlatformCapability interface {
	Join(elem ...string) string
	Abs(path string) (string, error)
	EnsureWithinWorkspace(path string) (string, error)
	ReadDir(path string) ([]fs.DirEntry, error)
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, content []byte) error
	Stat(path string) (fs.FileInfo, error)
}

// RiskEvaluator 是 tools 模块所需的风险评估最小接口。
//
// 不直接引用 risk 包内部类型，由 bootstrap 注入。
type RiskEvaluator interface {
	EvaluateOperation(operationName string, targetObject string) (riskLevel string, err error)
}

// AuditWriter 是 tools 模块所需的审计写入最小接口。
//
// 不直接引用 audit 包内部类型，由 bootstrap 注入。
type AuditWriter interface {
	WriteAuditRecord(taskID, runID, auditType, action, summary, target, result string) error
}

// ExecutionCapability 是 tools 模块所需的最小执行后端接口。
//
// 该接口用于受控命令执行工具，不直接暴露平台实现细节。
type ExecutionCapability interface {
	RunCommand(ctx context.Context, command string, args []string, workingDir string) (CommandExecutionResult, error)
}

// CommandExecutionResult 描述一次受控命令执行的最小输出。
type CommandExecutionResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// BrowserPageReadResult 描述浏览器页面读取的最小结果。
type BrowserPageReadResult struct {
	URL         string
	Title       string
	TextContent string
	MIMEType    string
	TextType    string
	Source      string
}

// BrowserPageSearchResult 描述页面内基础搜索的最小结果。
type BrowserPageSearchResult struct {
	URL        string
	Query      string
	MatchCount int
	Matches    []string
	Source     string
}

// PlaywrightSidecarClient 是 Playwright sidecar 的最小客户端边界。
type PlaywrightSidecarClient interface {
	ReadPage(ctx context.Context, url string) (BrowserPageReadResult, error)
	SearchPage(ctx context.Context, url, query string, limit int) (BrowserPageSearchResult, error)
}

// CheckpointService 是 tools 模块所需的恢复点最小接口。
//
// 不直接引用 checkpoint 包内部类型，由 bootstrap 注入。
type CheckpointService interface {
	CreateRecoveryPoint(taskID, summary string, objects []string) error
}

// ModelCapability 是 tools 模块所需的统一模型能力接口。
//
// 工具层只能通过这层接口访问模型接入，不得直接在工具实现里散落 SDK 调用。
// 具体实现由 internal/model.Service 提供，并由 bootstrap 注入到执行上下文。
type ModelCapability interface {
	GenerateText(ctx context.Context, request model.GenerateTextRequest) (model.GenerateTextResponse, error)
	Provider() string
	ModelID() string
}

// ToolExecuteContext 携带单次工具执行所需的全部运行时上下文。
//
// task_id / run_id / step_id 与协议层的 Task / Run / Step 对齐，
// trace_id 用于链路追踪。
// workspace_path 是当前工作区根路径，平台路径操作必须通过
// PlatformCapability 完成，不能直接拼接。
// logger 保留为 any 类型，避免引入具体日志库依赖，
// 工具实现按需做类型断言或通过简单接口使用。
// timeout 和 cancel 由 executor 在创建 context 时设置，
// 工具实现应尊重 ctx.Done() 信号。
// storage / platform / risk / audit / checkpoint 均为可选注入，
// 工具实现使用前需做 nil 检查，不得假设一定可用。
type ToolExecuteContext struct {
	TaskID               string
	RunID                string
	StepID               string
	TraceID              string
	WorkspacePath        string
	Logger               any
	Timeout              time.Duration
	Cancel               context.CancelFunc
	ApprovalGranted      bool
	ApprovedOperation    string
	ApprovedTargetObject string

	Storage    StorageCapability
	Platform   PlatformCapability
	Execution  ExecutionCapability
	Playwright PlaywrightSidecarClient
	Risk       RiskEvaluator
	Audit      AuditWriter
	Checkpoint CheckpointService
	Model      ModelCapability
}

// ---------------------------------------------------------------------------
// 错误类型
// ---------------------------------------------------------------------------

var (
	// ErrToolNameRequired 表示工具名称不能为空。
	ErrToolNameRequired = errors.New("tools: tool name is required")
	// ErrToolNameInvalid 表示工具名称不符合 snake_case 规范。
	ErrToolNameInvalid = errors.New("tools: tool name is invalid")
	// ErrToolSourceRequired 表示工具来源不能为空。
	ErrToolSourceRequired = errors.New("tools: tool source is required")
	// ErrToolSourceInvalid 表示工具来源不在允许范围内。
	ErrToolSourceInvalid = errors.New("tools: tool source is invalid")
	// ErrToolDisplayNameRequired 表示工具显示名称不能为空。
	ErrToolDisplayNameRequired = errors.New("tools: tool display_name is required")
	// ErrToolNotFound 表示请求的工具未在 registry 中注册。
	ErrToolNotFound = errors.New("tools: tool not found")
	// ErrToolValidationFailed 表示工具输入校验失败。
	ErrToolValidationFailed = errors.New("tools: tool validation failed")
	// ErrToolExecutionFailed 表示工具执行过程中发生错误。
	ErrToolExecutionFailed = errors.New("tools: tool execution failed")
	// ErrToolExecutionTimeout 表示工具执行超时。
	ErrToolExecutionTimeout = errors.New("tools: tool execution timeout")
	// ErrToolOutputInvalid indicates invalid tool output.
	ErrToolOutputInvalid = errors.New("tools: tool output invalid")
	// ErrWorkerNotAvailable indicates the worker is unavailable.
	ErrWorkerNotAvailable = errors.New("tools: worker not available")
	// ErrPlaywrightSidecarFailed indicates the Playwright sidecar failed.
	ErrPlaywrightSidecarFailed = errors.New("tools: playwright sidecar failed")
	// ErrToolDryRunNotSupported 表示工具不支持预检查模式。
	ErrToolDryRunNotSupported = errors.New("tools: tool dry run not supported")
	// ErrToolDuplicateName 表示注册时发现同名工具已存在。
	ErrToolDuplicateName = errors.New("tools: duplicate tool name")
	// ErrApprovalRequired 表示命中审批门禁，当前执行被阻塞。
	ErrApprovalRequired = errors.New("tools: approval required")
	// ErrWorkspaceBoundaryDenied 表示目标路径超出工作区边界。
	ErrWorkspaceBoundaryDenied = errors.New("tools: workspace boundary denied")
	// ErrCommandNotAllowed 表示命中了被拦截的危险命令。
	ErrCommandNotAllowed = errors.New("tools: command not allowed")
	// ErrCapabilityDenied 表示当前平台能力不足，无法安全执行。
	ErrCapabilityDenied = errors.New("tools: capability denied")
)

// ---------------------------------------------------------------------------
// 辅助函数
// ---------------------------------------------------------------------------

// isSnakeCase 判断字符串是否符合 snake_case 规范。
//
// 允许小写字母、数字和下划线，不允许大写字母、连字符或空格。
func isSnakeCase(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			continue
		}
		if c >= '0' && c <= '9' {
			if i == 0 {
				return false
			}
			continue
		}
		if c == '_' {
			if i == 0 || i == len(s)-1 {
				return false
			}
			continue
		}
		return false
	}
	return true
}
