## 2026-03-20 - Early Return on Canceled Context in Singleflight
**Learning:** In highly concurrent systems, acquiring a mutex lock when the context is already canceled or expired is a pure waste of resources. By checking `ctx.Err()` *before* lock acquisition in `Group.Do`, we can short-circuit the entire singleflight process for failed contexts.
**Action:** Always check `ctx.Err()` before engaging in expensive or blocking operations (like acquiring a Mutex) in context-aware functions. For generic returns, explicitly declare `var zero V` to avoid implicit runtime allocations compared to `*new(V)`.

## 2024-05-24 - Zero Allocation WaitGroup vs Channel
**Learning:** For wait groups in highly-contended scenarios, allocating a `chan struct{}` on the heap accounts for a measurable percentage of memory allocation on hot paths (nearly 5% memory usage from `make(chan struct{})`). When `context.Background()` or an uncancellable context is used, `ctx.Done() == nil`, allowing a `select` statement to be bypassed completely and avoiding the need for a channel.
**Action:** When synchronization is required but context cancellation is optional or unused, use an embedded `sync.WaitGroup` directly in the object struct and fallback to `chan struct{}` only when `ctx.Done() != nil`. This removes allocation completely on the fast path.

## 2026-03-26 - Delay Map Initialization for Hot-Path Map Reads
**Learning:** In Go, reading from a nil map is safe and evaluates to the zero-value and `ok=false`. Checking if a map is nil before a read lookup on a hot path adds an unnecessary branch instruction.
**Action:** When lazy-initializing a map in a highly concurrent struct (like singleflight), place the `if m == nil { m = make(map[...]) }` initialization *only* on the write path (Leader), and allow the read path (Follower) to cleanly fall through a nil map read.
