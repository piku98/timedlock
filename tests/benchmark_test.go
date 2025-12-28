package timedlock_test

import (
	"context"
	"testing"
	"time"

	"github.com/piku98/timedlock"
)

// BenchmarkTimedLock_Lock benchmarks basic lock acquisition and release.
func BenchmarkTimedLock_Lock(b *testing.B) {
	lock := timedlock.New()
	ctx := context.Background()
	timeout := 1 * time.Second

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lock.Lock(ctx, timeout)
		lock.Unlock()
	}
}

// BenchmarkTimedLock_TryLock benchmarks non-blocking lock attempts.
func BenchmarkTimedLock_TryLock(b *testing.B) {
	lock := timedlock.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if lock.TryLock() {
			lock.Unlock()
		}
	}
}

// BenchmarkTimedLock_Contention benchmarks lock performance under contention.
func BenchmarkTimedLock_Contention(b *testing.B) {
	lock := timedlock.New()
	ctx := context.Background()
	timeout := 100 * time.Millisecond

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := lock.Lock(ctx, timeout); err == nil {
				// Simulate very brief critical section
				lock.Unlock()
			}
		}
	})
}

// BenchmarkTimedLock_LockWithAutoRelease benchmarks lock with auto-release.
func BenchmarkTimedLock_LockWithAutoRelease(b *testing.B) {
	lock := timedlock.New()
	ctx := context.Background()
	acquireTimeout := 1 * time.Second
	releaseTimeout := 100 * time.Millisecond

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lock.LockWithAutoRelease(ctx, acquireTimeout, &releaseTimeout)
		// Wait for auto-release
		time.Sleep(110 * time.Millisecond)
	}
}

// BenchmarkTimedLock_ShortTimeout benchmarks with very short timeouts.
func BenchmarkTimedLock_ShortTimeout(b *testing.B) {
	lock := timedlock.New()
	ctx := context.Background()

	// Hold the lock
	_ = lock.Lock(ctx, 1*time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lock.Lock(ctx, 1*time.Nanosecond)
	}

	lock.Unlock()
}

// BenchmarkTimedLock_MultipleUnlock benchmarks the cost of redundant unlocks.
func BenchmarkTimedLock_MultipleUnlock(b *testing.B) {
	lock := timedlock.New()
	ctx := context.Background()

	_ = lock.Lock(ctx, 1*time.Second)
	lock.Unlock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lock.Unlock()
	}
}

// BenchmarkTimedLock_ContextCancellation benchmarks performance with cancelled context.
func BenchmarkTimedLock_ContextCancellation(b *testing.B) {
	lock := timedlock.New()

	// Hold the lock
	_ = lock.Lock(context.Background(), 1*time.Second)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Immediately cancel
		_ = lock.Lock(ctx, 1*time.Second)
	}

	lock.Unlock()
}

// BenchmarkTimedLock_RapidAcquireRelease benchmarks rapid lock cycling.
func BenchmarkTimedLock_RapidAcquireRelease(b *testing.B) {
	lock := timedlock.New()
	ctx := context.Background()
	timeout := 10 * time.Millisecond

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 10; j++ {
			_ = lock.Lock(ctx, timeout)
			lock.Unlock()
		}
	}
}

// BenchmarkTimedLock_CreateAndDestroy benchmarks lock creation overhead.
func BenchmarkTimedLock_CreateAndDestroy(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = timedlock.New()
	}
}
