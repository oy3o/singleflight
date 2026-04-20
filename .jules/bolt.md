## 2024-04-20 - Leveraging return parameter assignments in defers
**Learning:** Returning cached local variables and boolean flags from deferred functions provides an effective way to avoid later dereferencing heap-allocated struct fields, avoiding data races and saving memory reads.
**Action:** When working on concurrent deferred functions, use named return parameters to locally cache object state that is read after unlocking.
