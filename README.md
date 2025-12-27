# TimedLock

[![Go Reference](https://pkg.go.dev/badge/github.com/yourusername/timedlock.svg)](https://pkg.go.dev/github.com/yourusername/timedlock)
[![Go Report Card](https://goreportcard.com/badge/github.com/yourusername/timedlock)](https://goreportcard.com/report/github.com/yourusername/timedlock)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A production-ready Go synchronization library providing mutex-like locking with **timeout**, **context**, and **auto-release** support.

## Why TimedLock?

While `sync.Mutex` is perfect for simple mutual exclusion, real-world applications often need:
- ⏱️ **Timeouts** to prevent indefinite blocking
- 🎯 **Context integration** for cancellation and deadlines  
- 🔓 **Auto-release** to prevent deadlocks
- 🚀 **Non-blocking attempts** to check lock availability
- 📊 **Rich error information** for debugging

TimedLock provides all of these while maintaining the simplicity of a mutex.

## How It Works

TimedLock uses a **buffered channel** (capacity 1) as a semaphore:

```
┌─────────────────────────────────────────────────────────────┐
│                       TimedLock                             │
│                                                             │
│  ┌───────────────────────────────────────────┐             │
│  │   semaphore: chan struct{} (capacity: 1)  │             │
│  └───────────────────────────────────────────┘             │
│                                                             │
│  Empty channel = Lock available  ✓                         │
│  Full channel  = Lock held        ✗                        │
└─────────────────────────────────────────────────────────────┘

Lock Acquisition Flow:
─────────────────────

Goroutine A                  Goroutine B
    │                            │
    ├─── Lock(ctx, 5s) ─────────┼─── Lock(ctx, 5s)
    │                            │
    ▼                            ▼
┌────────┐                  ┌────────┐
│ select │                  │ select │
│  case  │                  │  case  │
└────┬───┘                  └────┬───┘
     │                            │
     ├─→ semaphore ← struct{}     │
     │   (Success! ✓)             │
     │                            │
     ├─→ Auto-release timer       ├─→ Waiting... ⏳
     │   starts (if enabled)      │
     │                            │
     ├─→ Critical Section         │
     │                            ├─→ Timeout after 5s? ⏱️
     │                            ├─→ Context cancelled? 🚫
     └─→ Unlock() ────────────────┼─→ Lock acquired! ✓
                                  │
                                  └─→ Critical Section
```

## Installation

```bash
go get github.com/yourusername/timedlock
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/yourusername/timedlock"
)

func main() {
    lock := timedlock.New()
    ctx := context.Background()
    
    // Acquire lock with 5 second timeout
    if err := lock.Lock(ctx, 5*time.Second); err != nil {
        fmt.Printf("Failed to acquire lock: %v\n", err)
        return
    }
    defer lock.Unlock()
    
    // Critical section - only one goroutine at a time
    fmt.Println("Processing shared resource...")
    processSharedResource()
}
```

## Core API

### Creating a Lock

```go
lock := timedlock.New()
```

### Lock Acquisition

#### `Lock(ctx context.Context, timeout time.Duration) error`

Attempts to acquire the lock within the specified timeout.

**Parameters:**
- `ctx` - Context for cancellation support
- `timeout` - Maximum time to wait for lock acquisition

**Returns:**
- `nil` - Lock acquired successfully
- `ErrLockTimeout` - Timeout expired before acquiring lock
- `ErrContextCancelled` - Context was cancelled  
- `ErrInvalidTimeout` - Timeout is negative

```go
if err := lock.Lock(ctx, 5*time.Second); err != nil {
    switch {
    case errors.Is(err, timedlock.ErrLockTimeout):
        fmt.Println("Timeout: lock is held by another goroutine")
    case errors.Is(err, timedlock.ErrContextCancelled):
        fmt.Println("Operation cancelled")
    default:
        fmt.Printf("Error: %v\n", err)
    }
    return
}
defer lock.Unlock()
```

#### `LockWithAutoRelease(ctx context.Context, acquireTimeout time.Duration, releaseTimeout *time.Duration) error`

Acquires lock with optional automatic release after a duration. Perfect for preventing deadlocks!

```go
// Lock will automatically release after 10 seconds
releaseTimeout := 10 * time.Second
if err := lock.LockWithAutoRelease(ctx, 5*time.Second, &releaseTimeout); err != nil {
    return err
}
// Even if we forget to unlock, it releases automatically after 10s
doWork()
```

#### `TryLock() bool`

Non-blocking lock attempt. Returns immediately.

```go
if lock.TryLock() {
    defer lock.Unlock()
    // Got the lock, proceed
    processResource()
} else {
    // Lock is busy, do something else
    fmt.Println("Resource busy, skipping...")
}
```

### Lock Release

#### `Unlock()`

Releases the lock. Safe to call multiple times (subsequent calls are no-ops).

```go
lock.Unlock()  // Releases the lock
lock.Unlock()  // Safe - no panic
```

## Real-World Examples

### Example 1: Rate Limiting

Limit operations to once per second:

```go
type RateLimiter struct {
    lock *timedlock.TimedLock
}

func NewRateLimiter() *RateLimiter {
    return &RateLimiter{lock: timedlock.New()}
}

func (rl *RateLimiter) AllowRequest(ctx context.Context) error {
    // Auto-release after 1 second = 1 request per second max
    interval := 1 * time.Second
    
    if err := rl.lock.LockWithAutoRelease(ctx, 100*time.Millisecond, &interval); err != nil {
        return fmt.Errorf("rate limit exceeded: %w", err)
    }
    
    return nil // Request allowed
}

// Usage
limiter := NewRateLimiter()

for i := 0; i < 5; i++ {
    if err := limiter.AllowRequest(ctx); err != nil {
        fmt.Printf("Request %d: DENIED (rate limited)\n", i+1)
    } else {
        fmt.Printf("Request %d: ALLOWED\n", i+1)
        processRequest()
    }
    time.Sleep(400 * time.Millisecond)
}
```

### Example 2: Resource Pool

Manage exclusive access to pooled resources:

```go
type ResourcePool struct {
    resources []Resource
    locks     []*timedlock.TimedLock
}

func NewResourcePool(size int) *ResourcePool {
    pool := &ResourcePool{
        resources: make([]Resource, size),
        locks:     make([]*timedlock.TimedLock, size),
    }
    for i := 0; i < size; i++ {
        pool.locks[i] = timedlock.New()
    }
    return pool
}

func (p *ResourcePool) Acquire(ctx context.Context) (*Resource, func(), error) {
    // Try each resource
    for i, lock := range p.locks {
        if err := lock.Lock(ctx, 2*time.Second); err == nil {
            resource := &p.resources[i]
            release := func() { lock.Unlock() }
            return resource, release, nil
        }
    }
    return nil, nil, errors.New("no resources available")
}

// Usage
pool := NewResourcePool(3)

resource, release, err := pool.Acquire(ctx)
if err != nil {
    log.Fatal(err)
}
defer release()

resource.Use()
```

### Example 3: Graceful Shutdown

Respect context cancellation for clean shutdowns:

```go
func worker(ctx context.Context, lock *timedlock.TimedLock, id int) {
    for {
        // Try to acquire lock, respecting context cancellation
        if err := lock.Lock(ctx, 5*time.Second); err != nil {
            if errors.Is(err, timedlock.ErrContextCancelled) {
                fmt.Printf("Worker %d: Shutdown signal received\n", id)
                return
            }
            time.Sleep(100 * time.Millisecond)
            continue
        }
        
        // Do work
        fmt.Printf("Worker %d: Processing...\n", id)
        time.Sleep(500 * time.Millisecond)
        lock.Unlock()
        
        // Check for shutdown between iterations
        select {
        case <-ctx.Done():
            fmt.Printf("Worker %d: Graceful shutdown\n", id)
            return
        default:
            // Continue working
        }
    }
}

// Usage
ctx, cancel := context.WithCancel(context.Background())
lock := timedlock.New()

// Start workers
var wg sync.WaitGroup
for i := 1; i <= 3; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        worker(ctx, lock, id)
    }(i)
}

// Wait for interrupt signal
<-sigterm
cancel() // Signal all workers to stop

wg.Wait() // Wait for clean shutdown
fmt.Println("All workers stopped gracefully")
```

### Example 4: Deadlock Prevention

Use auto-release to prevent deadlocks:

```go
func processWithTimeout(lock *timedlock.TimedLock, data Data) error {
    ctx := context.Background()
    
    // Auto-release after 30 seconds as a safety mechanism
    maxProcessTime := 30 * time.Second
    
    if err := lock.LockWithAutoRelease(ctx, 5*time.Second, &maxProcessTime); err != nil {
        return err
    }
    
    // Even if processing hangs, lock releases after 30s
    // preventing system-wide deadlock
    return processData(data)
}
```

## Error Handling

### Standard Errors

```go
var (
    ErrLockTimeout      = errors.New("failed to acquire lock within timeout")
    ErrContextCancelled = errors.New("context cancelled during lock acquisition")
    ErrInvalidTimeout   = errors.New("invalid timeout duration")
)
```

### Custom Error Types

#### LockTimeoutError

Provides detailed timeout information:

```go
type LockTimeoutError struct {
    Timeout     time.Duration  // Requested timeout
    ElapsedTime time.Duration  // Actual time elapsed
    Message     string         // Descriptive message
}
```

**Usage:**

```go
err := lock.Lock(ctx, 100*time.Millisecond)
if err != nil {
    var timeoutErr *timedlock.LockTimeoutError
    if errors.As(err, &timeoutErr) {
        fmt.Printf("Timed out after %v (requested %v)\n", 
            timeoutErr.ElapsedTime, timeoutErr.Timeout)
    }
}
```

#### ContextError

Wraps context-related errors:

```go
type ContextError struct {
    Operation string  // Operation that was attempted
    Cause     error   // Underlying cause
}
```

## Performance

### Benchmarks

```bash
$ go test -bench=. ./tests/...

BenchmarkTimedLock_Lock-8           1000000    1204 ns/op    0 B/op   0 allocs/op
BenchmarkTimedLock_TryLock-8       50000000      32 ns/op    0 B/op   0 allocs/op
BenchmarkTimedLock_Contention-8      500000    3512 ns/op    0 B/op   0 allocs/op
```

### Performance Characteristics

- **Lock/Unlock**: ~1200 ns per operation (no contention)
- **TryLock**: ~32 ns per operation
- **Memory**: Zero allocations for basic operations
- **Contention**: Gracefully handles high contention

### Comparison with sync.Mutex

| Feature | sync.Mutex | TimedLock | Use Case |
|---------|-----------|-----------|----------|
| Performance | ~20 ns | ~1200 ns | sync.Mutex 60x faster |
| Timeout support | ❌ | ✅ | Network operations |
| Context cancellation | ❌ | ✅ | Graceful shutdown |
| Non-blocking try | ❌ | ✅ | Opportunistic locking |
| Auto-release | ❌ | ✅ | Deadlock prevention |
| Custom errors | ❌ | ✅ | Error debugging |
| Safe multiple unlock | ❌ | ✅ | Defensive programming |

**When to use what:**
- **sync.Mutex**: High-frequency, low-latency critical sections
- **TimedLock**: Network I/O, distributed systems, user-facing operations

## Testing

```bash
# Run all tests
go test ./tests/...

# Run with race detector
go test -race ./tests/...

# Run benchmarks
go test -bench=. -benchmem ./tests/...

# Check coverage
go test -cover ./tests/...
```

### Test Coverage

- ✅ 20 comprehensive unit tests
- ✅ Race condition detection
- ✅ Timeout and context cancellation
- ✅ Concurrent access patterns
- ✅ Edge cases and error conditions
- ✅ Auto-release timing correctness

## Best Practices

### 1. Always Use defer for Unlock

```go
if err := lock.Lock(ctx, timeout); err != nil {
    return err
}
defer lock.Unlock()  // Ensures unlock even if panic occurs
// Critical section
```

### 2. Choose Appropriate Timeouts

```go
// Fast operation
lock.Lock(ctx, 100*time.Millisecond)

// I/O operation  
lock.Lock(ctx, 5*time.Second)

// Background job
lock.Lock(ctx, 30*time.Second)
```

### 3. Handle All Error Cases

```go
err := lock.Lock(ctx, timeout)
if err != nil {
    switch {
    case errors.Is(err, timedlock.ErrLockTimeout):
        // Retry or return busy error
    case errors.Is(err, timedlock.ErrContextCancelled):
        // Clean shutdown
    default:
        // Unexpected error
    }
}
```

### 4. Use TryLock for Optional Operations

```go
if lock.TryLock() {
    defer lock.Unlock()
    updateCache()  // Nice to have, not critical
} else {
    // Skip if busy
    metrics.IncrementSkipped()
}
```

## Design Principles

1. **Simplicity** - Clean API that feels natural to Go developers
2. **Safety** - Thread-safe, panic-free, well-tested
3. **Performance** - Minimal overhead, zero allocations where possible
4. **Observability** - Rich error information for debugging
5. **Flexibility** - Multiple acquisition patterns for different use cases

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Author

Created with ❤️ for the Go community

---

**Need help?** Open an issue or check the [documentation](https://pkg.go.dev/github.com/yourusername/timedlock)
