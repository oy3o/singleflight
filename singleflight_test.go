package singleflight

import (
	"context"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sync/singleflight"
)

// -----------------------------------------------------------------------------
// 模拟负载 (Mock Workload)
// -----------------------------------------------------------------------------

// expensiveWork 模拟一个耗时操作，例如 DB 查询或 RPC 调用
// 使用 atomic 避免单纯的 time.Sleep 被编译器优化或调度造成误差，
// 同时模拟一定的 CPU 消耗。
func expensiveWork() (string, error) {
	time.Sleep(time.Millisecond) // 模拟 1ms 的 I/O 延迟
	return "moonlight-value", nil
}

// -----------------------------------------------------------------------------
// 场景 1: 惊群效应 (Thundering Herd)
// 所有并发请求都打同一个 Key，测试锁竞争和 WaitGroup 的唤醒性能
// -----------------------------------------------------------------------------

func Benchmark_ThunderingHerd_Std(b *testing.B) {
	var g singleflight.Group
	key := "hot_key"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			v, _, _ := g.Do(key, func() (interface{}, error) {
				return expensiveWork()
			})
			// 标准库必须付出类型断言的代价
			if str, ok := v.(string); !ok || str != "moonlight-value" {
				// 这里的 panic 只是为了防止编译器优化掉返回值
				panic("mismatch")
			}
		}
	})
}

func Benchmark_ThunderingHerd_Moon(b *testing.B) {
	// 假设我们在同一个包下，或者已经正确引用了 Generic Group
	var g Group[string, string]
	key := "hot_key"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// 泛型版本直接返回 string，无装箱/拆箱开销
			v, _, _ := g.Do(context.Background(), key, func(ctx context.Context) (string, error) {
				return expensiveWork()
			})
			if v != "moonlight-value" {
				panic("mismatch")
			}
		}
	})
}

// -----------------------------------------------------------------------------
// 场景 2: 离散键值 (High Entropy / Cache Miss)
// 每个请求 Key 都不同，测试 Map 的写入性能和锁的粒度
// 这是标准库 sync.Mutex 最痛的地方
// -----------------------------------------------------------------------------

func Benchmark_RandomKeys_Std(b *testing.B) {
	var g singleflight.Group
	// 预生成 Key 避免 benchmark 中包含 fmt.Sprintf 的开销
	keys := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = strconv.Itoa(i)
	}

	var idx int64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := atomic.AddInt64(&idx, 1) - 1
			if i >= int64(len(keys)) {
				i = 0 // 循环安全
			}
			key := keys[i]

			v, _, _ := g.Do(key, func() (interface{}, error) {
				return expensiveWork()
			})
			if _, ok := v.(string); !ok {
				panic("mismatch")
			}
		}
	})
}

func Benchmark_RandomKeys_Moon(b *testing.B) {
	var g Group[string, string]
	keys := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		keys[i] = strconv.Itoa(i)
	}

	var idx int64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := atomic.AddInt64(&idx, 1) - 1
			if i >= int64(len(keys)) {
				i = 0
			}
			key := keys[i]

			// xsync.MapOf 的分段锁在这里应该会有巨大优势
			g.Do(context.Background(), key, func(ctx context.Context) (string, error) {
				return expensiveWork()
			})
		}
	})
}

// -----------------------------------------------------------------------------
// 场景 3: 极速 CPU 密集型 (Zero Allocation Check)
// 只有计算，没有 Sleep，测试纯粹的框架开销
// -----------------------------------------------------------------------------

func Benchmark_Overhead_Std(b *testing.B) {
	var g singleflight.Group
	key := "fast_key"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			g.Do(key, func() (interface{}, error) {
				return "fast", nil
			})
		}
	})
}

func Benchmark_Overhead_Moon(b *testing.B) {
	var g Group[string, string]
	key := "fast_key"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			g.Do(context.Background(), key, func(ctx context.Context) (string, error) {
				return "fast", nil
			})
		}
	})
}
