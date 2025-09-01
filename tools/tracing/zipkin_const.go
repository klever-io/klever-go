//go:build !test
// +build !test

package tracing

import "time"

// Circuit breaker constants
const (
	cbMaxFailures       = 5               // Open circuit after 5 consecutive failures
	cbHalfOpenRetries   = 1               // Number of test requests in half-open state
	cbInitialBackoff    = 1 * time.Second // Initial backoff duration for tests
	cbMaxBackoff        = 5 * time.Minute // Maximum backoff duration for tests
	cbBackoffMultiplier = 2               // Exponential backoff multiplier
)

// Default configuration constants
const (
	defaultBatchSize    = 100              // Default number of spans per batch
	defaultPushInterval = 5 * time.Second  // Default interval for pushing spans
	httpTimeout         = 10 * time.Second // HTTP client timeout
	maxRetries          = 3                // Maximum number of retry attempts for sending spans
	defaultCleanupRatio = 0.9              // Default cleanup ratio when memory limit exceeded
)
