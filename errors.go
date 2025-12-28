package timedlock

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrLockTimeout      = errors.New("failed to acquire lock within timeout")
	ErrLockNotHeld      = errors.New("lock is not currently held")
	ErrLockAlreadyHeld  = errors.New("lock is already held")
	ErrInvalidTimeout   = errors.New("invalid timeout duration")
	ErrContextCancelled = errors.New("context cancelled during lock acquisition")
)

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

func (e *LockTimeoutError) Unwrap() error {
	return ErrLockTimeout
}

func (e *LockTimeoutError) Is(target error) bool {
	return target == ErrLockTimeout
}

type ContextError struct {
	Operation string
	Cause     error
}

func (e *ContextError) Error() string {
	return fmt.Sprintf("context error during %s: %v", e.Operation, e.Cause)
}

func (e *ContextError) Unwrap() error {
	return e.Cause
}

func NewLockTimeoutError(timeout, elapsed time.Duration, message string) *LockTimeoutError {
	return &LockTimeoutError{
		Timeout:     timeout,
		ElapsedTime: elapsed,
		Message:     message,
	}
}

func NewContextError(operation string, cause error) *ContextError {
	return &ContextError{
		Operation: operation,
		Cause:     cause,
	}
}

