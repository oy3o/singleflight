## 2024-03-13 - [Fast-path Context done in Select Statements]
**Learning:** Using `select` in Go with a `ctx.Done()` adds non-trivial runtime overhead in hot-paths. If a context is not cancelable (like `context.Background()`), `ctx.Done()` returns `nil`. Checking for a nil channel and bypassing the `select` block using a direct channel receive (`<-done`) yields a measurable performance boost.
**Action:** Always check if a context channel is `nil` to fast-path blocking operations, bypassing `select` where feasible on critical synchronization primitives.

## 2024-03-14 - [Closing Channels outside critical sections]
**Learning:** Closing a channel wakes up all waiting goroutines. If this is done inside a `sync.Mutex` critical section, the awakened goroutines trigger the Go scheduler context switch. This heavily increases lock contention, blocking all other fast-paths and new operations from taking place until the context switch is resolved.
**Action:** In highly-concurrent pathways (like Singleflight leaders returning), capture the channel reference inside the lock, release the lock, and only then close the channel to prevent unnecessary scheduling pauses within the critical section.
