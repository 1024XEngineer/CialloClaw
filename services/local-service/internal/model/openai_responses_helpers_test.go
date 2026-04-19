package model

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestNormalizeOpenAIBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
		wantErr  error
	}{
		{
			name:     "trim responses suffix",
			endpoint: " https://api.openai.com/v1/responses ",
			want:     "https://api.openai.com/v1",
		},
		{
			name:     "preserve base url",
			endpoint: "https://api.openai.com/v1",
			want:     "https://api.openai.com/v1",
		},
		{
			name:     "reject blank endpoint",
			endpoint: "   ",
			wantErr:  ErrOpenAIEndpointRequired,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeOpenAIBaseURL(tc.endpoint)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("normalizeOpenAIBaseURL returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("base url mismatch: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestClassifyOpenAIRequestError(t *testing.T) {
	if err := classifyOpenAIRequestError(nil); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	timeoutErr := classifyOpenAIRequestError(context.DeadlineExceeded)
	if !errors.Is(timeoutErr, ErrOpenAIRequestTimeout) {
		t.Fatalf("expected timeout classification, got %v", timeoutErr)
	}

	netTimeoutErr := classifyOpenAIRequestError(timeoutError{})
	if !errors.Is(netTimeoutErr, ErrOpenAIRequestTimeout) {
		t.Fatalf("expected net timeout classification, got %v", netTimeoutErr)
	}

	decodeErr := classifyOpenAIRequestError(errors.New("invalid character 'x' looking for beginning of value"))
	if !errors.Is(decodeErr, ErrOpenAIResponseInvalid) {
		t.Fatalf("expected invalid response classification, got %v", decodeErr)
	}

	requestErr := classifyOpenAIRequestError(errors.New("connection reset by peer"))
	if !errors.Is(requestErr, ErrOpenAIRequestFailed) {
		t.Fatalf("expected request failure classification, got %v", requestErr)
	}
}

func TestOpenAIResponseHelpers(t *testing.T) {
	if got := firstNonEmpty("  ", "", "value", "other"); got != "value" {
		t.Fatalf("unexpected first non-empty value: %q", got)
	}

	defaultSchema := normalizeToolSchema(nil)
	if defaultSchema["type"] != "object" {
		t.Fatalf("expected default object schema, got %+v", defaultSchema)
	}
	if defaultSchema["additionalProperties"] != true {
		t.Fatalf("expected additionalProperties=true, got %+v", defaultSchema)
	}

	customSchema := map[string]any{"type": "string"}
	if got := normalizeToolSchema(customSchema); got["type"] != "string" {
		t.Fatalf("expected provided schema to be preserved, got %+v", got)
	}

	if got := truncateErrorMessage(strings.Repeat("a", 300)); len(got) != 256 {
		t.Fatalf("expected truncated error length 256, got %d", len(got))
	}
	if got := truncateErrorMessage(" short "); got != "short" {
		t.Fatalf("expected trimmed error message, got %q", got)
	}

	if !looksLikeJSONDecodeError(errors.New("unexpected end of JSON input")) {
		t.Fatal("expected json decode error detection to match")
	}
	if looksLikeJSONDecodeError(errors.New("connection refused")) {
		t.Fatal("expected non-json error not to match")
	}
}
