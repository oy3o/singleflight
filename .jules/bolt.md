## 2024-03-13 - [Fast-path Context done in Select Statements]
**Learning:** Using `select` in Go with a `ctx.Done()` adds non-trivial runtime overhead in hot-paths. If a context is not cancelable (like `context.Background()`), `ctx.Done()` returns `nil`. Checking for a nil channel and bypassing the `select` block using a direct channel receive (`<-done`) yields a measurable performance boost.
**Action:** Always check if a context channel is `nil` to fast-path blocking operations, bypassing `select` where feasible on critical synchronization primitives.
