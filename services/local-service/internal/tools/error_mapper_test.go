package tools

import (
	"fmt"
	"testing"
)

func TestDefaultToolErrorMapper(t *testing.T) {
	mapper := DefaultToolErrorMapper{}
	tests := []struct {
		name string
		err  error
		code int
		ok   bool
	}{
		{name: "tool_not_found", err: fmt.Errorf("wrap: %w", ErrToolNotFound), code: ToolErrorCodeNotFound, ok: true},
		{name: "execution_failed", err: fmt.Errorf("wrap: %w", ErrToolExecutionFailed), code: ToolErrorCodeExecutionFailed, ok: true},
		{name: "timeout", err: fmt.Errorf("wrap: %w", ErrToolExecutionTimeout), code: ToolErrorCodeTimeout, ok: true},
		{name: "output_invalid", err: fmt.Errorf("wrap: %w", ErrToolOutputInvalid), code: ToolErrorCodeOutputInvalid, ok: true},
		{name: "worker_unavailable", err: fmt.Errorf("wrap: %w", ErrWorkerNotAvailable), code: ToolErrorCodeWorkerNotAvailable, ok: true},
		{name: "sidecar_failed", err: fmt.Errorf("wrap: %w", ErrPlaywrightSidecarFailed), code: ToolErrorCodePlaywrightSidecarFail, ok: true},
		{name: "unknown", err: fmt.Errorf("no mapping"), code: 0, ok: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, ok := mapper.Map(tc.err)
			if ok != tc.ok || code != tc.code {
				t.Fatalf("expected (%d,%v), got (%d,%v)", tc.code, tc.ok, code, ok)
			}
		})
	}
}
