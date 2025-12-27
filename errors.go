package timedlock

import (
	"errors"
	"fmt"
	"time"
)

// Common errors that can be returned by TimedLock operations.
var (
	// ErrLockTimeout is returned when lock acquisition times out.
	ErrLockTimeout = errors.New("failed to acquire lock within timeout")

	// ErrLockNotHeld is returned when trying to unlock a lock that is not currently held.
	ErrLockNotHeld = errors.New("lock is not currently held")

	// ErrLockAlreadyHeld is returned when trying to acquire a lock that is already held by the caller.
	ErrLockAlreadyHeld = errors.New("lock is already held")

	// ErrInvalidTimeout is returned when an invalid timeout duration is provided.
	ErrInvalidTimeout = errors.New("invalid timeout duration")

	// ErrContextCancelled is returned when the context is cancelled during lock acquisition.
	ErrContextCancelled = errors.New("context cancelled during lock acquisition")
)

// LockTimeoutError provides detailed information about a lock timeout.
type LockTimeoutError struct {
	Timeout     time.Duration
	ElapsedTime time.Duration
	Message     string
}

func (e *LockTimeoutError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("lock timeout: %s (timeout: %v, elapsed: %v)",
			e.Message, e.Timeout, e.ElapsedTime)
	}
	return fmt.Sprintf("lock timeout: failed to acquire lock within %v (elapsed: %v)",
		e.Timeout, e.ElapsedTime)
}

// Unwrap returns the underlying error for error wrapping support.
func (e *LockTimeoutError) Unwrap() error {
	return ErrLockTimeout
}

// Is implements error matching for errors.Is.
func (e *LockTimeoutError) Is(target error) bool {
	return target == ErrLockTimeout
}

// ContextError wraps context-related errors with additional information.
type ContextError struct {
	Operation string
	Cause     error
}

func (e *ContextError) Error() string {
	return fmt.Sprintf("context error during %s: %v", e.Operation, e.Cause)
}

// Unwrap returns the underlying error.
func (e *ContextError) Unwrap() error {
	return e.Cause
}

// NewLockTimeoutError creates a new LockTimeoutError.
func NewLockTimeoutError(timeout, elapsed time.Duration, message string) *LockTimeoutError {
	return &LockTimeoutError{
		Timeout:     timeout,
		ElapsedTime: elapsed,
		Message:     message,
	}
}

// NewContextError creates a new ContextError.
func NewContextError(operation string, cause error) *ContextError {
	return &ContextError{
		Operation: operation,
		Cause:     cause,
	}
}

