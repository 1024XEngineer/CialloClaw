package orchestrator

import (
	"errors"
	"strings"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
)

func TestModelExecutionFailureBubbleCoversTypedFailures(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		message string
	}{
		{name: "tool calling unsupported", err: model.ErrToolCallingNotSupported, message: "不支持工具调用"},
		{name: "invalid response", err: model.ErrOpenAIResponseInvalid, message: "返回内容无法解析"},
		{name: "timeout", err: model.ErrOpenAIRequestTimeout, message: "模型请求超时"},
		{name: "request failed", err: model.ErrOpenAIRequestFailed, message: "模型请求发送失败"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bubble := modelExecutionFailureBubble(test.err)
			if !strings.Contains(bubble, test.message) {
				t.Fatalf("expected bubble %q to contain %q", bubble, test.message)
			}
		})
	}
}

func TestModelHTTPStatusFailureBubbleCoversStatusMatrix(t *testing.T) {
	tests := []struct {
		name    string
		err     *model.OpenAIHTTPStatusError
		message string
	}{
		{name: "nil", err: nil, message: ""},
		{name: "bad request with detail", err: &model.OpenAIHTTPStatusError{StatusCode: 400, Message: "input too large"}, message: "模型请求被上游拒绝（input too large）"},
		{name: "bad request generic", err: &model.OpenAIHTTPStatusError{StatusCode: 400, Message: "Authorization: Bearer sk-secret-value invalid"}, message: "模型请求被上游拒绝，请检查输入内容、模型能力和接口兼容性。"},
		{name: "auth with detail", err: &model.OpenAIHTTPStatusError{StatusCode: 401, Message: "missing scope"}, message: "模型鉴权失败（missing scope）"},
		{name: "auth redacted", err: &model.OpenAIHTTPStatusError{StatusCode: 403, Message: "Authorization: Bearer sk-secret-value invalid"}, message: "模型鉴权失败，请检查 API Key 或访问权限。"},
		{name: "not found", err: &model.OpenAIHTTPStatusError{StatusCode: 404, Message: "missing route"}, message: "模型接口不存在（missing route）"},
		{name: "not found generic", err: &model.OpenAIHTTPStatusError{StatusCode: 404, Message: ""}, message: "模型接口不存在，请检查 Base URL 或接口兼容性。"},
		{name: "timeout", err: &model.OpenAIHTTPStatusError{StatusCode: 408, Message: "timeout"}, message: "模型请求超时，请稍后重试。"},
		{name: "too many requests", err: &model.OpenAIHTTPStatusError{StatusCode: 429, Message: "rate limited"}, message: "模型请求过于频繁（rate limited）"},
		{name: "too many requests generic", err: &model.OpenAIHTTPStatusError{StatusCode: 429, Message: ""}, message: "模型请求过于频繁，请稍后重试。"},
		{name: "service unavailable", err: &model.OpenAIHTTPStatusError{StatusCode: 503, Message: "temporary outage"}, message: "模型服务暂时不可用（temporary outage）"},
		{name: "service unavailable generic", err: &model.OpenAIHTTPStatusError{StatusCode: 500, Message: ""}, message: "模型服务暂时不可用，请稍后重试。"},
		{name: "generic status", err: &model.OpenAIHTTPStatusError{StatusCode: 418, Message: "teapot"}, message: "模型调用失败（teapot）"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bubble := modelHTTPStatusFailureBubble(test.err)
			if bubble != "" || test.message != "" {
				if !strings.Contains(bubble, test.message) {
					t.Fatalf("expected bubble %q to contain %q", bubble, test.message)
				}
			}
		})
	}
}

func TestSanitizeModelProviderMessageNormalizesAndRedactsSecrets(t *testing.T) {
	secretMessage := sanitizeModelProviderMessage(" Authorization: Bearer sk-secret-value invalid ")
	if secretMessage != "" {
		t.Fatalf("expected secret-bearing provider message to be redacted, got %q", secretMessage)
	}

	normalized := sanitizeModelProviderMessage("  line one\n line two\r\nline three  ")
	if normalized != "line one line two line three" {
		t.Fatalf("expected whitespace normalization, got %q", normalized)
	}

	longMessage := sanitizeModelProviderMessage(strings.Repeat("trimme ", 30))
	if len(longMessage) == 0 || len(longMessage) > 123 || !strings.HasSuffix(longMessage, "...") {
		t.Fatalf("expected truncation with ellipsis, got %q", longMessage)
	}
}

func TestExecutionFailureBubbleFallsBackToGenericForUnknownErrors(t *testing.T) {
	bubble := executionFailureBubble(errors.New("boom"))
	if bubble != "执行失败：请稍后重试。" {
		t.Fatalf("expected generic fallback bubble, got %q", bubble)
	}
}
