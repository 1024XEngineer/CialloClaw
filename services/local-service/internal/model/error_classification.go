package model

import (
	"errors"
	"net/http"
)

// IsProviderRuntimeUnavailable reports whether the current provider failure is a
// transient runtime problem that callers should treat as retryable or
// temporarily unavailable instead of as a permanent configuration mismatch.
func IsProviderRuntimeUnavailable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrOpenAIRequestFailed) || errors.Is(err, ErrOpenAIRequestTimeout) || errors.Is(err, ErrOpenAIResponseInvalid) {
		return true
	}

	var statusErr *OpenAIHTTPStatusError
	if !errors.As(err, &statusErr) {
		return false
	}
	return statusErr.StatusCode == http.StatusTooManyRequests || (statusErr.StatusCode >= 500 && statusErr.StatusCode <= 599)
}
