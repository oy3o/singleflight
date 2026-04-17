## 2024-06-25 - Avoid Heap Memory Accesses and Data Races with Multiple Returns
**Learning:** Returning cached local variables and boolean flags from deferred functions when unlocking instead of accessing struct fields saves overhead by avoiding heap read penalties and prevents race conditions without holding a lock in the parent function.
**Action:** Always favor multi-return local state over deferred updates read outside the lock.
