// Package timedlock provides synchronization primitives for concurrent programming.
//
// The main type is TimedLock, which provides a mutex-like lock with timeout support
// for both acquisition and automatic release.
package timedlock

import (
	"context"
	"time"
)

// TimedLock is a synchronization primitive that provides mutual exclusion with timeout support.
// It allows setting timeouts for both lock acquisition and automatic release.
//
// Unlike sync.Mutex, TimedLock:
//   - Supports timeout-based lock acquisition
//   - Supports automatic lock release after a duration
//   - Is context-aware for cancellation support
//   - Provides non-blocking TryLock operations
//
// TimedLock is safe for concurrent use by multiple goroutines.
type TimedLock struct {
	semaphore chan struct{}
}

// New creates and returns a new TimedLock instance.
//
// Example:
//
//	lock := timedlock.New()
//	defer lock.Unlock()
//
//	if err := lock.Lock(ctx, 5*time.Second); err != nil {
//	    log.Printf("Failed to acquire lock: %v", err)
//	    return err
//	}
//	// critical section
func New() *TimedLock {
	return &TimedLock{
		semaphore: make(chan struct{}, 1),
	}
}

// Lock attempts to acquire the lock within the specified timeout.
// It blocks until either the lock is acquired or the timeout expires.
//
// Parameters:
//   - ctx: Context for cancellation support
//   - acquireTimeout: Maximum duration to wait for lock acquisition
//
// Returns:
//   - nil if lock is successfully acquired
//   - ErrLockTimeout if timeout expires before acquiring lock
//   - ErrContextCancelled if context is cancelled
//   - ErrInvalidTimeout if timeout is negative
//
// Example:
//
//	if err := lock.Lock(ctx, 5*time.Second); err != nil {
//	    if errors.Is(err, timedlock.ErrLockTimeout) {
//	        // Handle timeout
//	    }
//	    return err
//	}
//	defer lock.Unlock()
func (t *TimedLock) Lock(ctx context.Context, acquireTimeout time.Duration) error {
	return t.LockWithAutoRelease(ctx, acquireTimeout, nil)
}

// LockWithAutoRelease attempts to acquire the lock with optional automatic release.
// This is useful for scenarios where you want to ensure the lock is released even if
// the holder crashes or forgets to unlock.
//
// Parameters:
//   - ctx: Context for cancellation support
//   - acquireTimeout: Maximum duration to wait for lock acquisition
//   - releaseTimeout: Optional duration after which lock is automatically released
//
// Returns:
//   - nil if lock is successfully acquired
//   - ErrLockTimeout if timeout expires before acquiring lock
//   - ErrContextCancelled if context is cancelled
//   - ErrInvalidTimeout if any timeout is negative
//
// Example:
//
//	// Acquire lock with auto-release after 10 seconds
//	releaseTimeout := 10 * time.Second
//	if err := lock.LockWithAutoRelease(ctx, 5*time.Second, &releaseTimeout); err != nil {
//	    return err
//	}
//	// Lock will be automatically released after 10 seconds
func (t *TimedLock) LockWithAutoRelease(
	ctx context.Context,
	acquireTimeout time.Duration,
	releaseTimeout *time.Duration,
) error {
	if acquireTimeout < 0 {
		return ErrInvalidTimeout
	}

	if releaseTimeout != nil && *releaseTimeout < 0 {
		return ErrInvalidTimeout
	}

	start := time.Now()
	acquireCtx, cancel := context.WithTimeout(ctx, acquireTimeout)
	defer cancel()

	select {
	case <-acquireCtx.Done():
		elapsed := time.Since(start)
		if ctx.Err() != nil {
			return NewContextError("lock acquisition", ctx.Err())
		}
		return NewLockTimeoutError(acquireTimeout, elapsed, "lock is held by another goroutine")
	case t.semaphore <- struct{}{}:
		// Lock acquired successfully - now setup auto-release if specified
		if releaseTimeout != nil {
			go func() {
				releaseCtx, releaseCancel := context.WithTimeout(context.Background(), *releaseTimeout)
				defer releaseCancel()
				<-releaseCtx.Done()
				t.Unlock()
			}()
		}
		return nil
	}
}

// TryLock attempts to acquire the lock without blocking.
// It returns immediately whether or not the lock was acquired.
//
// Returns:
//   - true if lock was successfully acquired
//   - false if lock is already held
//
// Example:
//
//	if lock.TryLock() {
//	    defer lock.Unlock()
//	    // critical section
//	} else {
//	    // Lock is busy, do something else
//	}
func (t *TimedLock) TryLock() bool {
	select {
	case t.semaphore <- struct{}{}:
		return true
	default:
		return false
	}
}

// Unlock releases the lock, allowing other goroutines to acquire it.
// It is safe to call Unlock on an already unlocked lock (it will be a no-op).
//
// Best practice is to use defer:
//
//	if err := lock.Lock(ctx, timeout); err != nil {
//	    return err
//	}
//	defer lock.Unlock()
func (t *TimedLock) Unlock() {
	select {
	case <-t.semaphore:
	default:
		// Lock not held, no-op
	}
}
