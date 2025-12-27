package timedlock_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"timedlock"
)

// Test 1: Lock and auto-release with releaseTimeout
func TestTimedLock_AutoReleaseAfterTimeout(t *testing.T) {
	lock := timedlock.New()
	ctx := context.Background()

	releaseTimeout := 100 * time.Millisecond
	acquireTimeout := 50 * time.Millisecond

	// Acquire lock with auto-release
	err := lock.LockWithAutoRelease(ctx, acquireTimeout, &releaseTimeout)
	if err != nil {
		t.Fatalf("Expected to acquire lock, got error: %v", err)
	}

	// Wait for auto-release to happen
	time.Sleep(150 * time.Millisecond)

	// Try to acquire again - should succeed if auto-release worked
	err = lock.Lock(ctx, acquireTimeout)
	if err != nil {
		t.Fatalf("Expected lock to be released and re-acquired, got error: %v", err)
	}
	lock.Unlock() // Clean up
}

// Test 2: Wait and acquire lock after it's released
func TestTimedLock_AcquireAfterRelease(t *testing.T) {
	lock := timedlock.New()
	ctx := context.Background()

	// First goroutine acquires lock and releases after 100ms
	err := lock.Lock(ctx, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Expected to acquire lock, got error: %v", err)
	}

	// Release after 100ms
	go func() {
		time.Sleep(100 * time.Millisecond)
		lock.Unlock()
	}()

	// Second acquisition should wait and succeed within 200ms
	start := time.Now()
	err = lock.Lock(ctx, 200*time.Millisecond)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Expected to acquire lock after waiting, got error: %v", err)
	}

	if duration < 90*time.Millisecond {
		t.Errorf("Expected to wait at least ~100ms, but only waited %v", duration)
	}

	if duration > 150*time.Millisecond {
		t.Errorf("Expected to acquire lock quickly after release, but took %v", duration)
	}
	lock.Unlock() // Clean up
}

// Test 3: Timeout if unable to acquire lock
func TestTimedLock_TimeoutWhenUnableToAcquire(t *testing.T) {
	lock := timedlock.New()
	ctx := context.Background()

	// First goroutine holds the lock
	err := lock.Lock(ctx, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Expected to acquire lock, got error: %v", err)
	}

	// Second goroutine tries to acquire with short timeout and should fail
	start := time.Now()
	err = lock.Lock(ctx, 100*time.Millisecond)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error when unable to acquire lock, but got nil")
	}

	if !errors.Is(err, timedlock.ErrLockTimeout) {
		t.Errorf("Expected ErrLockTimeout, got: %v", err)
	}

	// Check if it's a LockTimeoutError with details
	var timeoutErr *timedlock.LockTimeoutError
	if errors.As(err, &timeoutErr) {
		if timeoutErr.Timeout != 100*time.Millisecond {
			t.Errorf("Expected timeout of 100ms, got %v", timeoutErr.Timeout)
		}
		t.Logf("Timeout details: %s", timeoutErr.Error())
	}

	if duration < 90*time.Millisecond || duration > 150*time.Millisecond {
		t.Errorf("Expected to wait ~100ms before timeout, but waited %v", duration)
	}

	lock.Unlock() // Clean up
}

// Test 4: Manual unlock works correctly
func TestTimedLock_ManualUnlock(t *testing.T) {
	lock := timedlock.New()
	ctx := context.Background()

	// Acquire lock without auto-release
	err := lock.Lock(ctx, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Expected to acquire lock, got error: %v", err)
	}

	// Manually unlock
	lock.Unlock()

	// Should be able to acquire again immediately
	err = lock.Lock(ctx, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Expected to re-acquire lock after manual unlock, got error: %v", err)
	}
	lock.Unlock() // Clean up
}

// Test 5: Context cancellation during acquisition
func TestTimedLock_ContextCancellation(t *testing.T) {
	lock := timedlock.New()
	ctx, cancel := context.WithCancel(context.Background())

	// Hold the lock
	err := lock.Lock(context.Background(), 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Expected to acquire lock, got error: %v", err)
	}

	// Cancel context after 50ms
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	// Try to acquire with cancelled context
	start := time.Now()
	err = lock.Lock(ctx, 200*time.Millisecond)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected error due to context cancellation")
	}

	var ctxErr *timedlock.ContextError
	if errors.As(err, &ctxErr) {
		t.Logf("Context error: %s", ctxErr.Error())
	}

	// Should timeout around 50ms due to context cancellation
	if duration > 100*time.Millisecond {
		t.Errorf("Expected to fail quickly after context cancellation, but took %v", duration)
	}

	lock.Unlock() // Clean up
}

// Test 6: Multiple concurrent acquisitions
func TestTimedLock_MultipleConcurrentAcquisitions(t *testing.T) {
	lock := timedlock.New()
	ctx := context.Background()

	type result struct {
		success bool
	}
	results := make(chan result, 5)

	// Start 5 goroutines trying to acquire the lock
	for i := 0; i < 5; i++ {
		go func(id int) {
			err := lock.Lock(ctx, 50*time.Millisecond)
			if err == nil {
				time.Sleep(20 * time.Millisecond) // Hold lock briefly
				lock.Unlock()
				results <- result{success: true}
			} else {
				results <- result{success: false}
			}
		}(i)
	}

	// Wait for all goroutines and count results
	successCount := 0
	timeoutCount := 0
	for i := 0; i < 5; i++ {
		res := <-results
		if res.success {
			successCount++
		} else {
			timeoutCount++
		}
	}

	// At least one should succeed
	if successCount == 0 {
		t.Error("Expected at least one goroutine to acquire the lock")
	}

	t.Logf("Success: %d, Timeout: %d", successCount, timeoutCount)
}

// Test 7: Sequential lock acquisitions
func TestTimedLock_SequentialAcquisitions(t *testing.T) {
	lock := timedlock.New()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		err := lock.Lock(ctx, 50*time.Millisecond)
		if err != nil {
			t.Fatalf("Iteration %d: Expected to acquire lock, got error: %v", i, err)
		}
		lock.Unlock()
	}
}

// Test 8: Zero timeout should fail immediately if lock is held
func TestTimedLock_ZeroTimeout(t *testing.T) {
	lock := timedlock.New()
	ctx := context.Background()

	// Acquire lock
	err := lock.Lock(ctx, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Expected to acquire lock, got error: %v", err)
	}

	// Try to acquire with zero timeout
	start := time.Now()
	err = lock.Lock(ctx, 0)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected immediate timeout with zero duration")
	}

	// Should fail almost immediately (within a few milliseconds)
	if duration > 10*time.Millisecond {
		t.Errorf("Expected immediate failure, but took %v", duration)
	}

	lock.Unlock() // Clean up
}

// Test 9: Very long acquire timeout allows eventual acquisition
func TestTimedLock_LongTimeout(t *testing.T) {
	lock := timedlock.New()
	ctx := context.Background()

	// Acquire lock and release after 50ms
	err := lock.Lock(ctx, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("Expected to acquire lock, got error: %v", err)
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		lock.Unlock()
	}()

	// Try to acquire with long timeout - should succeed
	err = lock.Lock(ctx, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("Expected to acquire lock with long timeout, got error: %v", err)
	}
	lock.Unlock() // Clean up
}

// Test 10: Auto-release doesn't affect if lock is manually released first
func TestTimedLock_ManualReleaseBeforeAutoRelease(t *testing.T) {
	lock := timedlock.New()
	ctx := context.Background()

	releaseTimeout := 200 * time.Millisecond
	err := lock.LockWithAutoRelease(ctx, 50*time.Millisecond, &releaseTimeout)
	if err != nil {
		t.Fatalf("Expected to acquire lock, got error: %v", err)
	}

	// Manually unlock before auto-release
	lock.Unlock()

	// Should be able to acquire immediately
	err = lock.Lock(ctx, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Expected to re-acquire lock after manual unlock, got error: %v", err)
	}
	lock.Unlock() // Clean up

	// Wait for auto-release timeout to pass
	time.Sleep(250 * time.Millisecond)
}

// Test 11: Immediate acquisition when lock is free
func TestTimedLock_ImmediateAcquisition(t *testing.T) {
	lock := timedlock.New()
	ctx := context.Background()

	start := time.Now()
	err := lock.Lock(ctx, 100*time.Millisecond)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Expected immediate acquisition, got error: %v", err)
	}

	// Should acquire almost instantly
	if duration > 10*time.Millisecond {
		t.Errorf("Expected immediate acquisition, but took %v", duration)
	}
	lock.Unlock() // Clean up
}

// Test 12: Auto-release with very short timeout
func TestTimedLock_VeryShortAutoRelease(t *testing.T) {
	lock := timedlock.New()
	ctx := context.Background()

	releaseTimeout := 10 * time.Millisecond
	err := lock.LockWithAutoRelease(ctx, 50*time.Millisecond, &releaseTimeout)
	if err != nil {
		t.Fatalf("Expected to acquire lock, got error: %v", err)
	}

	// Wait for auto-release
	time.Sleep(30 * time.Millisecond)

	// Should be able to acquire again
	err = lock.Lock(ctx, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Expected lock to be auto-released, got error: %v", err)
	}
	lock.Unlock() // Clean up
}

// Test 13: Stress test - rapid acquisitions and releases
func TestTimedLock_RapidAcquisitionRelease(t *testing.T) {
	lock := timedlock.New()
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		err := lock.Lock(ctx, 10*time.Millisecond)
		if err != nil {
			t.Fatalf("Iteration %d: Failed to acquire lock: %v", i, err)
		}
		lock.Unlock()
	}
}

// Test 14: Parent context cancellation affects acquire timeout
func TestTimedLock_ParentContextCancellation(t *testing.T) {
	lock := timedlock.New()
	parentCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Hold the lock in another "session"
	err := lock.Lock(context.Background(), 20*time.Millisecond)
	if err != nil {
		t.Fatalf("Expected to acquire lock, got error: %v", err)
	}

	// Try to acquire with parent context that will timeout
	start := time.Now()
	err = lock.Lock(parentCtx, 200*time.Millisecond)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error from parent context")
	}

	// Should respect the shorter parent context timeout (~50ms)
	if duration < 40*time.Millisecond || duration > 100*time.Millisecond {
		t.Errorf("Expected to timeout around 50ms, but took %v", duration)
	}

	lock.Unlock() // Clean up
}

// Test 15: TryLock succeeds when lock is free
func TestTimedLock_TryLock_Success(t *testing.T) {
	lock := timedlock.New()

	if !lock.TryLock() {
		t.Fatal("Expected TryLock to succeed on free lock")
	}

	lock.Unlock()

	// Should be able to acquire again after unlock
	if !lock.TryLock() {
		t.Error("Expected to acquire lock again after unlock")
	}
	lock.Unlock()
}

// Test 16: TryLock fails when lock is held
func TestTimedLock_TryLock_Failure(t *testing.T) {
	lock := timedlock.New()
	ctx := context.Background()

	// Acquire lock
	err := lock.Lock(ctx, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Expected to acquire lock, got error: %v", err)
	}

	// TryLock should fail
	if lock.TryLock() {
		t.Fatal("Expected TryLock to fail on held lock")
	}

	lock.Unlock()
}

// Test 17: Multiple Unlock calls are safe
func TestTimedLock_MultipleUnlockSafe(t *testing.T) {
	lock := timedlock.New()
	ctx := context.Background()

	err := lock.Lock(ctx, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Expected to acquire lock, got error: %v", err)
	}

	// Multiple unlocks should be safe
	lock.Unlock()
	lock.Unlock()
	lock.Unlock()

	// Should still be able to acquire
	err = lock.Lock(ctx, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Expected to acquire lock after multiple unlocks, got error: %v", err)
	}
	lock.Unlock()
}

// Test 18: Negative timeout returns error
func TestTimedLock_NegativeTimeout(t *testing.T) {
	lock := timedlock.New()
	ctx := context.Background()

	err := lock.Lock(ctx, -1*time.Second)
	if err == nil {
		t.Fatal("Expected error for negative timeout")
	}

	if !errors.Is(err, timedlock.ErrInvalidTimeout) {
		t.Errorf("Expected ErrInvalidTimeout, got: %v", err)
	}
}

// Test 19: Auto-release should NOT trigger if lock acquisition fails
// EXPECTED BEHAVIOR: If lock acquisition times out, the auto-release should not run.
// BUG: Currently auto-release goroutine starts before lock is acquired, so it runs even on failure.
func TestTimedLock_AutoRelease_ShouldNotTriggerOnFailedAcquisition(t *testing.T) {
	lock := timedlock.New()
	ctx := context.Background()

	// First, acquire the lock and hold it
	err := lock.Lock(ctx, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to acquire initial lock: %v", err)
	}

	// Try to acquire with auto-release, but this will FAIL (timeout)
	releaseTimeout := 100 * time.Millisecond
	acquireTimeout := 50 * time.Millisecond

	err = lock.LockWithAutoRelease(ctx, acquireTimeout, &releaseTimeout)
	if err == nil {
		t.Fatal("Expected lock acquisition to fail, but it succeeded")
	}
	if !errors.Is(err, timedlock.ErrLockTimeout) {
		t.Errorf("Expected ErrLockTimeout, got: %v", err)
	}

	// CRITICAL TEST: Wait past the releaseTimeout period
	time.Sleep(150 * time.Millisecond)

	// The original lock should STILL be held (not released by the failed attempt's auto-release)
	if lock.TryLock() {
		lock.Unlock()
		t.Fatal("FAIL: Lock was released by auto-release from a FAILED acquisition attempt!")
	}

	// Clean up
	lock.Unlock()
}

// Test 20: Auto-release timer should start AFTER lock is acquired, not before
// EXPECTED BEHAVIOR: Auto-release timeout is measured from when lock is ACQUIRED.
// BUG: Currently timer starts when function is called, not when lock is acquired.
func TestTimedLock_AutoRelease_TimerStartsAfterAcquisition(t *testing.T) {
	lock := timedlock.New()
	ctx := context.Background()

	// Hold lock for 200ms
	err := lock.Lock(ctx, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to acquire initial lock: %v", err)
	}

	// Release after 200ms
	go func() {
		time.Sleep(200 * time.Millisecond)
		lock.Unlock()
	}()

	// Call LockWithAutoRelease at T=0
	// Lock will be acquired at T=200ms
	// Auto-release should happen at T=200ms + 300ms = T=500ms
	releaseTimeout := 300 * time.Millisecond
	acquireTimeout := 500 * time.Millisecond

	callTime := time.Now()
	err = lock.LockWithAutoRelease(ctx, acquireTimeout, &releaseTimeout)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	acquireTime := time.Now()
	waitTime := acquireTime.Sub(callTime)
	t.Logf("Lock acquired after waiting %v", waitTime)

	// Expected: Lock should be held for 300ms AFTER acquisition
	// So it should be released at acquireTime + 300ms

	// Wait for 250ms after acquisition (lock should still be held)
	time.Sleep(250 * time.Millisecond)

	// Lock should STILL be held (we're only 250ms into the 300ms hold period)
	if lock.TryLock() {
		lock.Unlock()
		t.Fatal("FAIL: Lock was released too early! Auto-release timer started before acquisition.")
	}

	t.Log("Lock still held at 250ms after acquisition (correct)")

	// Wait another 100ms (total 350ms after acquisition)
	time.Sleep(100 * time.Millisecond)

	// Now lock should be released (we're past the 300ms hold period)
	if !lock.TryLock() {
		t.Fatal("FAIL: Lock should have been released by now (350ms after acquisition)")
	}
	lock.Unlock()

	t.Log("Lock released at ~350ms after acquisition (correct)")
}

