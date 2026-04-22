package model

import (
	"fmt"
	"testing"
)

func TestIsProviderRuntimeUnavailable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "timeout", err: ErrOpenAIRequestTimeout, want: true},
		{name: "request failed", err: ErrOpenAIRequestFailed, want: true},
		{name: "response invalid", err: ErrOpenAIResponseInvalid, want: true},
		{name: "rate limited", err: &OpenAIHTTPStatusError{StatusCode: 429}, want: true},
		{name: "provider unavailable", err: &OpenAIHTTPStatusError{StatusCode: 503}, want: true},
		{name: "non retryable status", err: &OpenAIHTTPStatusError{StatusCode: 400}, want: false},
		{name: "unsupported provider", err: ErrModelProviderUnsupported, want: false},
		{name: "wrapped timeout", err: fmt.Errorf("wrapped: %w", ErrOpenAIRequestTimeout), want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsProviderRuntimeUnavailable(test.err); got != test.want {
				t.Fatalf("IsProviderRuntimeUnavailable() = %v, want %v", got, test.want)
			}
		})
	}
}
