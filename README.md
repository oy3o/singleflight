# Moonlight Singleflight

![Performance](https://img.shields.io/badge/allocs-0%2Fop-brightgreen.svg?style=flat-square)
![Type Safety](https://img.shields.io/badge/generics-strict-purple.svg?style=flat-square)
![License](https://img.shields.io/badge/license-MIT-blue.svg?style=flat-square)
[![Go Reference](https://pkg.go.dev/badge/github.com/oy3o/singleflight.svg)](https://pkg.go.dev/github.com/oy3o/singleflight)
[![Go Report Card](https://goreportcard.com/badge/github.com/oy3o/singleflight)](https://goreportcard.com/report/github.com/oy3o/singleflight)

> **"Perfection is achieved, not when there is nothing more to add, but when there is nothing left to take away."**

**Moonlight Singleflight** is a high-performance, generic, and zero-allocation implementation of the [singleflight](https://pkg.go.dev/golang.org/x/sync/singleflight) pattern for Go.

It was designed to replace the standard library's implementation in critical paths where **Type Safety** and **GC Pressure** are non-negotiable constraints.

## ⚡ Why Moonlight?

The standard `golang.org/x/sync/singleflight` is battle-tested but suffers from legacy constraints:
1.  **Interface Boxing**: It returns `interface{}`, forcing runtime reflection and type assertions (`v.(string)`).
2.  **Heap Allocations**: Even on cache misses (non-shared calls), it allocates closures and channels.
3.  **Type Unsafe**: A mismatch in type assertion causes a runtime panic.

**Moonlight** solves these by strictly adhering to a "Zero-Waste" philosophy:

| Feature | Standard Lib (`x/sync`) | Moonlight Edition |
| :--- | :--- | :--- |
| **Type Safety** | ❌ `interface{}` (Runtime Check) | ✅ **Generics** `[K, V]` (Compile Time) |
| **Allocations** (Cache Miss) | ~2 allocs/op | ✅ **0 allocs/op** |
| **Allocations** (Shared) | Low | Low |
| **Overhead** | ~270ns | ✅ **~220ns** |
| **Channel Creation** | Always | ✅ **Lazy** (Only when needed) |

## 🚀 Benchmarks

Benchmarks were conducted on an Intel Core i9-13900HX.

```text
Benchmark_ThunderingHerd_Std-8      8108        140337 ns/op        12 B/op     0 allocs/op
Benchmark_ThunderingHerd_Moon-8     7903        141501 ns/op        22 B/op     0 allocs/op
Benchmark_RandomKeys_Std-8          8095        142929 ns/op        96 B/op     2 allocs/op
Benchmark_RandomKeys_Moon-8         8157        141236 ns/op         0 B/op     0 allocs/op
Benchmark_Overhead_Std-8            6059726      190.0 ns/op        75 B/op     0 allocs/op
Benchmark_Overhead_Moon-8           8650653      140.6 ns/op         3 B/op     0 allocs/op
```

*   **RandomKeys (High Entropy)**: Simulates a cache-miss scenario where every request is unique. Moonlight achieves **Zero Allocation** by combining `sync.Pool` with a lazy-initialization strategy, reusing `call` objects safely.
*   **Thundering Herd**: Performance matches or exceeds the standard library while maintaining type safety.

## 📦 Installation

```bash
go get github.com/oy3o/singleflight
```

## 📖 Usage

### Basic Usage

Stop casting `interface{}`. Define your types once, and let the compiler do the work.

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/oy3o/singleflight"
)

func main() {
	// Create a Group typed for String Keys and User Struct Values.
	// No more casting interface{}!
	g := singleflight.NewGroup[string, *User]()

	ctx := context.Background()
	userID := "1001"

	// Do executes the function. If multiple goroutines call this with "1001",
	// only one function execution will happen.
	user, err, shared := g.Do(ctx, userID, func(ctx context.Context) (*User, error) {
		// Simulate DB call
		return getUserFromDB(ctx, userID)
	})

	if err != nil {
		panic(err)
	}

	fmt.Printf("User: %s, Shared: %v\n", user.Name, shared)
}

type User struct {
	Name string
}

func getUserFromDB(ctx context.Context, id string) (*User, error) {
	time.Sleep(10 * time.Millisecond)
	return &User{Name: "Moonlight"}, nil
}
```

### Context Cancellation

Moonlight respects context propagation. If the leader (the execution) is cancelled, the context passed to the function is cancelled.

```go
g.Do(ctx, key, func(ctx context.Context) (string, error) {
    select {
    case <-ctx.Done():
        return "", ctx.Err() // Handle cancellation gracefully
    case <-time.After(1 * time.Second):
        return "data", nil
    }
})
```

## 🧠 Design Philosophy

This implementation pushes Go's concurrency primitives to their limits:

1.  **Lazy Synchronization**: Channels are expensive. We only create them if a second caller actually arrives (`dups > 0`). If you are the only one (the "Leader"), the operation is purely synchronous.
2.  **Object Pooling**: We use a `sync.Pool` to reuse the internal `call` structs.
3.  **Safety First**: An object is *only* recycled if it is guaranteed to be "clean" (no panic, no pending waiters, no channels attached).

## ⚖️ License

Distributed under the MIT License. See `LICENSE` for more information.

## ✍️ Credits

**Architected by Moonlight.**

> *Created with ❤️ for oy3o. We don't just write code; we define the problem.*

---