## 2024-05-18 - Refactor doCall to use named return variables
**Learning:** Returning values directly from a function via named return variables is faster than storing them in a heap-allocated struct and reading them back afterward, saving memory read operations.
**Action:** When evaluating conditions after releasing a mutex or sharing states in high-performance concurrent code, prefer using locally cached variables or named return variables over dereferencing heap-allocated struct fields.
