// Package tools defines backend tool capability boundaries and shared carriers.
//
// This package owns the tool registry, adapters, executor facade, built-in
// tools, and worker/sidecar client boundaries. It does not own intent
// recognition, orchestrator/runengine state machines, delivery_result assembly,
// or frontend protocol consumption.
//
// Tool names use snake_case, outputs must remain mappable to /packages/protocol,
// each execution must produce a ToolCall record, and platform behavior must be
// injected through a platform adapter.
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
// Tool source classification.
// ---------------------------------------------------------------------------

// ToolSource classifies the capability layer used by registry and executor code.
type ToolSource string

const (
	// ToolSourceBuiltin marks local built-in tools executed in-process.
	ToolSourceBuiltin ToolSource = "builtin"
	// ToolSourceWorker marks tools invoked through a worker process.
	ToolSourceWorker ToolSource = "worker"
	// ToolSourceSidecar marks tools invoked through a sidecar process.
	ToolSourceSidecar ToolSource = "sidecar"
)

// ---------------------------------------------------------------------------
// Tool metadata.
// ---------------------------------------------------------------------------

// ToolMetadata describes static metadata for one registered tool.
//
// name must be snake_case, globally unique, and aligned with ToolCall.tool_name.
// display_name is a user-facing label, not the registry key. description is a
// short discovery hint. source marks builtin/worker/sidecar ownership. risk_hint
// aligns with shared risk_level semantics. timeout_sec uses executor defaults
// when zero. input_schema_ref and output_schema_ref reference /packages/protocol
// schemas without parsing them here. supports_dry_run advertises precheck support.
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

// Validate enforces required metadata fields and tool-name format.
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
// Tool execution results.
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
	CreatedAt  string         `json:"created_at"`
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

// ArtifactRef is a module-local reference to an artifact produced by tool work.
// It does not replace the protocol Artifact shape; callers map it to the formal
// delivery boundary.
type ArtifactRef struct {
	ArtifactType string `json:"artifact_type"`
	Title        string `json:"title"`
	Path         string `json:"path"`
	MimeType     string `json:"mime_type"`
}

// ToolResultError is normalized error information returned by tool execution.
type ToolResultError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// ---------------------------------------------------------------------------
// Tool interface.
// ---------------------------------------------------------------------------

// Tool is the core interface implemented by every executable tool.
//
// Metadata returns static registry data. Validate performs business-level input
// checks before execution. Execute performs tool work with ToolExecuteContext and
// raw input, then returns normalized ToolResult data.
//
// Execute must provide all data required for ToolCall recording, must not advance
// task/run/step/event state machines directly, must not assemble delivery_result
// directly, and must not return unregistered ad hoc JSON as a formal result.
type Tool interface {
	Metadata() ToolMetadata
	Validate(input map[string]any) error
	Execute(ctx context.Context, execCtx *ToolExecuteContext, input map[string]any) (*ToolResult, error)
}

// DryRunTool is implemented by tools that support precheck mode.
type DryRunTool interface {
	Tool
	DryRun(ctx context.Context, execCtx *ToolExecuteContext, input map[string]any) (*ToolResult, error)
}

// ---------------------------------------------------------------------------
// Tool execution context.
// ---------------------------------------------------------------------------

// StorageCapability is the minimal storage dependency required by tools.
// It avoids compile-time coupling to storage internals; bootstrap injects the
// concrete adapter.
type StorageCapability interface {
	DatabasePath() string
}

// PlatformCapability is the minimal platform dependency required by tools.
// It avoids compile-time coupling to platform internals; bootstrap injects the
// concrete adapter.
type PlatformCapability interface {
	Join(elem ...string) string
	Abs(path string) (string, error)
	EnsureWithinWorkspace(path string) (string, error)
	ReadDir(path string) ([]fs.DirEntry, error)
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, content []byte) error
	Stat(path string) (fs.FileInfo, error)
}

// RiskEvaluator is the minimal risk-evaluation boundary required by tools.
type RiskEvaluator interface {
	EvaluateOperation(operationName string, targetObject string) (riskLevel string, err error)
}

// AuditWriter is the minimal audit-write boundary required by tools.
type AuditWriter interface {
	WriteAuditRecord(taskID, runID, auditType, action, summary, target, result string) error
}

// ExecutionCapability is the minimal controlled-command execution backend.
type ExecutionCapability interface {
	RunCommand(ctx context.Context, command string, args []string, workingDir string) (CommandExecutionResult, error)
}

// CommandExecutionResult is the minimal output from one controlled command.
type CommandExecutionResult struct {
	Stdout           string
	Stderr           string
	ExitCode         int
	ExecutionBackend string
	SandboxContainer string
	SandboxImage     string
	Interrupted      bool
}

// BrowserAttachMode classifies how one real-browser request attaches to a
// user-owned browser session.
type BrowserAttachMode string

const (
	// BrowserAttachModeCDP routes browser control through a local Chromium CDP
	// endpoint that the user has already enabled.
	BrowserAttachModeCDP BrowserAttachMode = "cdp"
)

// BrowserAttachTarget describes the minimum target filters that keep one
// attach request scoped to the user's current browser state.
type BrowserAttachTarget struct {
	URL           string `json:"url,omitempty"`
	TitleContains string `json:"title_contains,omitempty"`
	PageIndex     *int   `json:"page_index,omitempty"`
}

// BrowserAttachConfig carries the normalized attach contract shared by page_*
// attach flows and browser_* actions.
type BrowserAttachConfig struct {
	Mode        BrowserAttachMode   `json:"mode,omitempty"`
	BrowserKind string              `json:"browser_kind,omitempty"`
	EndpointURL string              `json:"endpoint_url,omitempty"`
	Target      BrowserAttachTarget `json:"target,omitempty"`
}

// BrowserExecutionMetadata captures the transport details that explain whether
// a browser result came from a fresh launch or a user-owned attached session.
type BrowserExecutionMetadata struct {
	Attached         bool
	BrowserKind      string
	BrowserTransport string
	EndpointURL      string
}

// BrowserPageReadResult is the minimal output of one browser page read.
type BrowserPageReadResult struct {
	BrowserExecutionMetadata
	URL         string
	Title       string
	TextContent string
	MIMEType    string
	TextType    string
	Source      string
}

// BrowserPageSearchResult is the minimal output of one browser page search.
type BrowserPageSearchResult struct {
	BrowserExecutionMetadata
	URL        string
	Query      string
	MatchCount int
	Matches    []string
	Source     string
}

// BrowserPageInteractResult describes one page interaction run.
type BrowserPageInteractResult struct {
	BrowserExecutionMetadata
	URL            string
	Title          string
	TextContent    string
	ActionsApplied int
	Source         string
}

// BrowserStructuredDOMResult describes a structured DOM snapshot.
type BrowserStructuredDOMResult struct {
	BrowserExecutionMetadata
	URL      string
	Title    string
	Headings []string
	Links    []string
	Buttons  []string
	Inputs   []string
	Source   string
}

// BrowserAttachedPageResult identifies one attached browser tab selection.
type BrowserAttachedPageResult struct {
	BrowserExecutionMetadata
	PageIndex int
	Title     string
	URL       string
	Source    string
}

// BrowserTabInfo is the minimal attached tab summary returned by list calls.
type BrowserTabInfo struct {
	PageIndex int
	Title     string
	URL       string
}

// BrowserTabsListResult describes the visible tabs on one attached browser.
type BrowserTabsListResult struct {
	BrowserExecutionMetadata
	TabCount int
	Tabs     []BrowserTabInfo
	Source   string
}

// BrowserSnapshotResult describes a text-rich snapshot of the currently
// attached browser tab.
type BrowserSnapshotResult struct {
	BrowserAttachedPageResult
	TextContent string
	Headings    []string
	Links       []string
	Buttons     []string
	Inputs      []string
}

// BrowserNavigateRequest is the normalized request shape for attached browser
// navigation.
type BrowserNavigateRequest struct {
	Attach BrowserAttachConfig
	URL    string
}

// BrowserNavigationResult describes the loaded page after an attached browser
// navigation succeeds.
type BrowserNavigationResult struct {
	BrowserAttachedPageResult
	TextContent string
	MIMEType    string
	TextType    string
}

// BrowserInteractRequest is the normalized request shape for attached browser
// interactions.
type BrowserInteractRequest struct {
	Attach  BrowserAttachConfig
	Actions []map[string]any
}

// OCRTextResult describes OCR or plain text extraction output.
type OCRTextResult struct {
	Path      string
	Text      string
	Language  string
	Source    string
	PageCount int
}

// MediaTranscodeResult describes one media transcode or normalization result.
type MediaTranscodeResult struct {
	InputPath  string
	OutputPath string
	Format     string
	Source     string
}

// MediaFrameExtractResult describes extracted frame metadata.
type MediaFrameExtractResult struct {
	InputPath  string
	OutputDir  string
	FramePaths []string
	FrameCount int
	Source     string
}

// ScreenCaptureMode classifies one capture request shape.
type ScreenCaptureMode string

const (
	ScreenCaptureModeScreenshot ScreenCaptureMode = "screenshot"
	ScreenCaptureModeKeyframe   ScreenCaptureMode = "keyframe"
	ScreenCaptureModeClip       ScreenCaptureMode = "clip"
)

// ScreenAuthorizationState describes the effective authorization state of one
// screen capture session.
type ScreenAuthorizationState string

const (
	ScreenAuthorizationPending ScreenAuthorizationState = "pending"
	ScreenAuthorizationGranted ScreenAuthorizationState = "granted"
	ScreenAuthorizationDenied  ScreenAuthorizationState = "denied"
	ScreenAuthorizationExpired ScreenAuthorizationState = "expired"
	ScreenAuthorizationEnded   ScreenAuthorizationState = "ended"
)

// ScreenRetentionPolicy describes how long screen-derived material may live.
type ScreenRetentionPolicy string

const (
	ScreenRetentionTemporary ScreenRetentionPolicy = "temporary"
	ScreenRetentionReview    ScreenRetentionPolicy = "review"
	ScreenRetentionArtifact  ScreenRetentionPolicy = "artifact"
)

// ScreenSessionStartInput is the minimal input required to create one managed
// screen capture session.
type ScreenSessionStartInput struct {
	SessionID         string
	TaskID            string
	RunID             string
	Source            string
	Scope             string
	CaptureMode       ScreenCaptureMode
	AuthorizationHint string
	TTL               time.Duration
}

// ScreenSessionState is the normalized session carrier shared by platform,
// execution, and cleanup logic.
type ScreenSessionState struct {
	ScreenSessionID    string
	SessionID          string
	TaskID             string
	RunID              string
	Source             string
	Scope              string
	CaptureMode        ScreenCaptureMode
	AuthorizationState ScreenAuthorizationState
	CreatedAt          time.Time
	ExpiresAt          time.Time
	EndedAt            *time.Time
	TerminalReason     string
}

// ScreenCaptureInput describes one controlled screenshot or keyframe request.
type ScreenCaptureInput struct {
	ScreenSessionID string
	TaskID          string
	RunID           string
	CaptureMode     ScreenCaptureMode
	Source          string
	SourcePath      string
	Reason          string
	AllowPersist    bool
}

// ScreenFrameCandidate is the storage-agnostic output of one screen capture.
// It remains a candidate until a later lifecycle step promotes it into an
// artifact.
type ScreenFrameCandidate struct {
	FrameID           string
	ScreenSessionID   string
	TaskID            string
	RunID             string
	CaptureMode       ScreenCaptureMode
	Source            string
	Path              string
	CapturedAt        time.Time
	IsKeyframe        bool
	DedupeFingerprint string
	RetentionPolicy   ScreenRetentionPolicy
	CleanupRequired   bool
}

// KeyframeCaptureResult describes one keyframe sampling decision.
type KeyframeCaptureResult struct {
	Candidate         ScreenFrameCandidate
	Promoted          bool
	PromotionReason   string
	DedupeFingerprint string
}

// ScreenCleanupInput describes one cleanup request for session-bound or
// expired screen artifacts.
type ScreenCleanupInput struct {
	ScreenSessionID string
	Reason          string
	Paths           []string
	ExpiredBefore   time.Time
}

// ScreenCleanupResult summarizes one cleanup run.
type ScreenCleanupResult struct {
	ScreenSessionID string
	Reason          string
	DeletedPaths    []string
	SkippedPaths    []string
	DeletedCount    int
	SkippedCount    int
}

// PlaywrightSidecarClient defines the minimal browser-facing boundary exposed
// by the Playwright sidecar runtime.
type PlaywrightSidecarClient interface {
	ReadPage(ctx context.Context, url string) (BrowserPageReadResult, error)
	ReadPageAttached(ctx context.Context, url string, attach BrowserAttachConfig) (BrowserPageReadResult, error)
	SearchPage(ctx context.Context, url, query string, limit int) (BrowserPageSearchResult, error)
	SearchPageAttached(ctx context.Context, url, query string, limit int, attach BrowserAttachConfig) (BrowserPageSearchResult, error)
	InteractPage(ctx context.Context, url string, actions []map[string]any) (BrowserPageInteractResult, error)
	InteractPageAttached(ctx context.Context, url string, actions []map[string]any, attach BrowserAttachConfig) (BrowserPageInteractResult, error)
	StructuredDOM(ctx context.Context, url string) (BrowserStructuredDOMResult, error)
	StructuredDOMAttached(ctx context.Context, url string, attach BrowserAttachConfig) (BrowserStructuredDOMResult, error)
	AttachCurrentPage(ctx context.Context, attach BrowserAttachConfig) (BrowserAttachedPageResult, error)
	SnapshotBrowser(ctx context.Context, attach BrowserAttachConfig) (BrowserSnapshotResult, error)
	NavigateBrowser(ctx context.Context, request BrowserNavigateRequest) (BrowserNavigationResult, error)
	ListBrowserTabs(ctx context.Context, attach BrowserAttachConfig) (BrowserTabsListResult, error)
	FocusBrowserTab(ctx context.Context, attach BrowserAttachConfig) (BrowserAttachedPageResult, error)
	InteractBrowser(ctx context.Context, request BrowserInteractRequest) (BrowserPageInteractResult, error)
}

// OCRWorkerClient is the minimal OCR worker client boundary.
type OCRWorkerClient interface {
	ExtractText(ctx context.Context, path string) (OCRTextResult, error)
	OCRImage(ctx context.Context, path, language string) (OCRTextResult, error)
	OCRPDF(ctx context.Context, path, language string) (OCRTextResult, error)
}

// MediaWorkerClient is the minimal media worker client boundary.
type MediaWorkerClient interface {
	TranscodeMedia(ctx context.Context, inputPath, outputPath, format string) (MediaTranscodeResult, error)
	NormalizeRecording(ctx context.Context, inputPath, outputPath string) (MediaTranscodeResult, error)
	ExtractFrames(ctx context.Context, inputPath, outputDir string, everySeconds float64, limit int) (MediaFrameExtractResult, error)
}

// ScreenCaptureClient is the minimal owner-5 screen capture capability
// boundary. It intentionally models session, capture, and cleanup concerns
// without freezing any frontend-facing protocol shape.
type ScreenCaptureClient interface {
	StartSession(ctx context.Context, input ScreenSessionStartInput) (ScreenSessionState, error)
	GetSession(ctx context.Context, screenSessionID string) (ScreenSessionState, error)
	StopSession(ctx context.Context, screenSessionID, reason string) (ScreenSessionState, error)
	ExpireSession(ctx context.Context, screenSessionID, reason string) (ScreenSessionState, error)
	CaptureScreenshot(ctx context.Context, input ScreenCaptureInput) (ScreenFrameCandidate, error)
	CaptureKeyframe(ctx context.Context, input ScreenCaptureInput) (KeyframeCaptureResult, error)
	CleanupSessionArtifacts(ctx context.Context, input ScreenCleanupInput) (ScreenCleanupResult, error)
	CleanupExpiredScreenTemps(ctx context.Context, input ScreenCleanupInput) (ScreenCleanupResult, error)
}

// CheckpointService is the minimal recovery-point boundary required by tools.
type CheckpointService interface {
	CreateRecoveryPoint(taskID, summary string, objects []string) error
}

// ModelCapability is the unified model boundary exposed to tool implementations.
// Tool code must use this interface instead of calling provider SDKs directly.
type ModelCapability interface {
	GenerateText(ctx context.Context, request model.GenerateTextRequest) (model.GenerateTextResponse, error)
	Provider() string
	ModelID() string
}

// ToolExecuteContext carries all runtime state needed for one tool execution.
//
// TaskID, RunID, and StepID align with protocol Task/Run/Step identities. TraceID
// supports trace correlation. WorkspacePath is the workspace root; path work must
// go through PlatformCapability. Logger remains any to avoid binding the tools
// package to a logging library. Timeout and Cancel are executor-owned, and tools
// must honor ctx.Done(). Optional dependencies must be nil-checked before use.
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
	OCR        OCRWorkerClient
	Media      MediaWorkerClient
	Screen     ScreenCaptureClient
	Risk       RiskEvaluator
	Audit      AuditWriter
	Checkpoint CheckpointService
	Model      ModelCapability
}

// ---------------------------------------------------------------------------
// Error values.
// ---------------------------------------------------------------------------

var (
	// ErrToolNameRequired reports a missing tool name.
	ErrToolNameRequired = errors.New("tools: tool name is required")
	// ErrToolNameInvalid reports a tool name that is not snake_case.
	ErrToolNameInvalid = errors.New("tools: tool name is invalid")
	// ErrToolSourceRequired reports a missing tool source.
	ErrToolSourceRequired = errors.New("tools: tool source is required")
	// ErrToolSourceInvalid reports an unsupported tool source.
	ErrToolSourceInvalid = errors.New("tools: tool source is invalid")
	// ErrToolDisplayNameRequired reports a missing display name.
	ErrToolDisplayNameRequired = errors.New("tools: tool display_name is required")
	// ErrToolNotFound reports a registry lookup miss.
	ErrToolNotFound = errors.New("tools: tool not found")
	// ErrToolValidationFailed reports invalid tool input.
	ErrToolValidationFailed = errors.New("tools: tool validation failed")
	// ErrToolExecutionFailed reports a tool execution failure.
	ErrToolExecutionFailed = errors.New("tools: tool execution failed")
	// ErrToolExecutionTimeout reports a tool execution timeout.
	ErrToolExecutionTimeout = errors.New("tools: tool execution timeout")
	// ErrToolOutputInvalid indicates invalid tool output.
	ErrToolOutputInvalid = errors.New("tools: tool output invalid")
	// ErrWorkerNotAvailable indicates the worker is unavailable.
	ErrWorkerNotAvailable = errors.New("tools: worker not available")
	// ErrPlaywrightSidecarFailed indicates the Playwright sidecar failed.
	ErrPlaywrightSidecarFailed = errors.New("tools: playwright sidecar failed")
	// ErrOCRWorkerFailed indicates the OCR worker failed.
	ErrOCRWorkerFailed = errors.New("tools: ocr worker failed")
	// ErrMediaWorkerFailed indicates the media worker failed.
	ErrMediaWorkerFailed = errors.New("tools: media worker failed")
	// ErrScreenCaptureUnauthorized indicates the screen capture request lacks authorization.
	ErrScreenCaptureUnauthorized = errors.New("tools: screen capture unauthorized")
	// ErrScreenCaptureSessionExpired indicates the capture session is expired.
	ErrScreenCaptureSessionExpired = errors.New("tools: screen capture session expired")
	// ErrScreenCaptureNotSupported indicates the current platform cannot capture screens.
	ErrScreenCaptureNotSupported = errors.New("tools: screen capture not supported")
	// ErrScreenCaptureFailed indicates screenshot capture failed.
	ErrScreenCaptureFailed = errors.New("tools: screen capture failed")
	// ErrScreenKeyframeSamplingFailed indicates keyframe sampling failed.
	ErrScreenKeyframeSamplingFailed = errors.New("tools: screen keyframe sampling failed")
	// ErrScreenCleanupFailed indicates temporary screen artifact cleanup failed.
	ErrScreenCleanupFailed = errors.New("tools: screen artifact cleanup failed")
	// ErrToolDryRunNotSupported reports a tool that cannot run in precheck mode.
	ErrToolDryRunNotSupported = errors.New("tools: tool dry run not supported")
	// ErrToolDuplicateName reports a duplicate registry name.
	ErrToolDuplicateName = errors.New("tools: duplicate tool name")
	// ErrApprovalRequired reports an execution blocked by authorization gates.
	ErrApprovalRequired = errors.New("tools: approval required")
	// ErrWorkspaceBoundaryDenied reports a target outside the workspace boundary.
	ErrWorkspaceBoundaryDenied = errors.New("tools: workspace boundary denied")
	// ErrCommandNotAllowed reports a blocked dangerous command.
	ErrCommandNotAllowed = errors.New("tools: command not allowed")
	// ErrCapabilityDenied reports missing platform capability for safe execution.
	ErrCapabilityDenied = errors.New("tools: capability denied")
)

// ---------------------------------------------------------------------------
// Helpers.
// ---------------------------------------------------------------------------

// isSnakeCase reports whether s uses lower snake_case with digits after the
// first character.
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
