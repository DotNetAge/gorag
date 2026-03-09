package retry

import "strings"

// IsRetryableError checks if an error is retryable based on common patterns
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	retryableErrors := []string{
		"rate limit",
		"timeout",
		"temporary",
		"connection refused",
		"service unavailable",
		"deadline exceeded",
		"server error",
		"connection",
	}

	for _, retryable := range retryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}

	return false
}
