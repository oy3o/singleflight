## 2026-04-21 - [sync.Pool micro-optimization]
**Learning:** While Go automatically zero-initializes newly allocated objects (`new(T)`), replacing straight-line unconditional zeroing assignments with an `else` block (to skip assignments on new objects but apply them to pooled objects) is an unmeasurable micro-optimization due to branch prediction and pipeline overhead.
**Action:** Prefer straight-line unconditional assignments for simple field resets when reusing from `sync.Pool` unless the reset is exceptionally expensive. Measure such micro-optimizations carefully before committing to them.
