package tools

import "errors"

const (
	ToolErrorCodeNotFound              = 1003001
	ToolErrorCodeExecutionFailed       = 1003002
	ToolErrorCodeTimeout               = 1003003
	ToolErrorCodeOutputInvalid         = 1003004
	ToolErrorCodeApprovalRequired      = 1004001
	ToolErrorCodeWorkspaceDenied       = 1004003
	ToolErrorCodeCommandNotAllowed     = 1004004
	ToolErrorCodeCapabilityDenied      = 1004005
	ToolErrorCodeWorkerNotAvailable    = 1006001
	ToolErrorCodePlaywrightSidecarFail = 1006002
	ToolErrorCodeOCRWorkerFailed       = 1006003
	ToolErrorCodeMediaWorkerFailed     = 1006004
)

// ToolErrorMapper maps normalized tool errors to protocol error codes.
type ToolErrorMapper interface {
	Map(err error) (int, bool)
}

// DefaultToolErrorMapper provides the built-in error-code mapping.
type DefaultToolErrorMapper struct{}

// Map implements ToolErrorMapper.
func (DefaultToolErrorMapper) Map(err error) (int, bool) {
	switch {
	case err == nil:
		return 0, false
	case errors.Is(err, ErrToolNotFound):
		return ToolErrorCodeNotFound, true
	case errors.Is(err, ErrToolExecutionTimeout):
		return ToolErrorCodeTimeout, true
	case errors.Is(err, ErrToolOutputInvalid):
		return ToolErrorCodeOutputInvalid, true
	case errors.Is(err, ErrApprovalRequired):
		return ToolErrorCodeApprovalRequired, true
	case errors.Is(err, ErrWorkspaceBoundaryDenied):
		return ToolErrorCodeWorkspaceDenied, true
	case errors.Is(err, ErrCommandNotAllowed):
		return ToolErrorCodeCommandNotAllowed, true
	case errors.Is(err, ErrCapabilityDenied):
		return ToolErrorCodeCapabilityDenied, true
	case errors.Is(err, ErrWorkerNotAvailable):
		return ToolErrorCodeWorkerNotAvailable, true
	case errors.Is(err, ErrPlaywrightSidecarFailed):
		return ToolErrorCodePlaywrightSidecarFail, true
	case errors.Is(err, ErrOCRWorkerFailed):
		return ToolErrorCodeOCRWorkerFailed, true
	case errors.Is(err, ErrMediaWorkerFailed):
		return ToolErrorCodeMediaWorkerFailed, true
	case errors.Is(err, ErrToolExecutionFailed):
		return ToolErrorCodeExecutionFailed, true
	default:
		return 0, false
	}
}

func mapToolErrorCode(mapper ToolErrorMapper, err error) *int {
	if mapper == nil || err == nil {
		return nil
	}
	code, ok := mapper.Map(err)
	if !ok {
		return nil
	}
	return &code
}
