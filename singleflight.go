package singleflight

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
)

// Group 是 singleflight 的泛型实现，支持零值初始化。
// 相比标准库 x/sync/singleflight，提供三个差异化能力：
//   - 泛型：消除 interface{} 的装箱/断言开销
//   - Context：Follower 可因 context 取消而提前退出
//   - sync.Pool：在无 Follower 的快路径上复用 call 对象
type Group[K comparable, V any] struct {
	mu    sync.Mutex
	calls map[K]*call[V]
	pool  sync.Pool
}

type call[V any] struct {
	wg sync.WaitGroup

	val V
	err error

	panicErr *panicError

	// done 仅在有可取消 context 的 Follower 加入时才分配（懒初始化）。
	// Leader 独占或仅有 Background context 时保持 nil，避免 channel 分配（~96 bytes）。
	done chan struct{}

	dups int

	// shared 在持锁期间由 doCall 设置，
	// 避免 Leader 返回时无锁读 dups 导致的 data race。
	shared bool

	forgotten bool
}

// Do 对同一个 key 只允许一个 fn 在执行（Leader），
// 后续调用者（Follower）阻塞等待并共享结果。
//
// shared 表示结果是否被多个调用者共享。
func (g *Group[K, V]) Do(
	ctx context.Context,
	key K,
	fn func(ctx context.Context) (V, error),
) (v V, err error, shared bool) {

	// 已取消的 context 不值得进入临界区。
	if err := ctx.Err(); err != nil {
		var zero V
		return zero, err, false
	}

	g.mu.Lock()

	// Follower 路径
	if c, ok := g.calls[key]; ok {
		c.dups++

		// context.Background() 的 Done() 返回 nil，
		// 此时无须支持 context 取消，直接使用 WaitGroup 等待，完全避免 channel 分配。
		if doneCh := ctx.Done(); doneCh == nil {
			g.mu.Unlock()
			c.wg.Wait()
		} else {
			if c.done == nil {
				c.done = make(chan struct{})
			}
			done := c.done
			g.mu.Unlock()

			select {
			case <-done:
			case <-doneCh:
				// Follower 提前退出，必须递减 dups，
				// 否则 Leader 的 shared 判断和 pool 回收逻辑都会出错。
				g.mu.Lock()
				c.dups--
				g.mu.Unlock()
				var zero V
				return zero, ctx.Err(), true
			}
		}

		// panic 必须传播给每个 Follower，保持与标准库一致的语义。
		if c.panicErr != nil {
			panic(c.panicErr)
		}
		return c.val, c.err, true
	}

	//  Leader 路径

	// 支持零值初始化：首次使用时分配 map。
	if g.calls == nil {
		g.calls = make(map[K]*call[V])
	}

	// 从 pool 复用 call 对象。不设置 pool.New，
	// 因为 Get 返回 nil 时直接 new 比闭包更轻。
	c, _ := g.pool.Get().(*call[V])
	if c == nil {
		c = new(call[V])
	} else {
		// 仅对从 pool 中复用的旧对象重置状态，避免对新对象产生冗余内存写入开销
		c.dups = 0
		c.forgotten = false
		c.panicErr = nil
		c.shared = false
	}
	c.wg.Add(1)
	// c.done 在回收前已被置为 nil，无需重置。

	g.calls[key] = c
	g.mu.Unlock()

	g.doCall(c, key, fn, ctx)

	val := c.val
	err = c.err
	shared = c.shared
	panicked := c.panicErr != nil

	// 仅当无 Follower 且无 panic 时回收。
	// 有 Follower 意味着 done channel 已分配且 Follower 可能仍在读 c.val，
	// 此时回收会导致 use-after-free。
	// 使用 !shared 避免对 c.dups 的内存重读。
	if !panicked && !shared && c.done == nil {
		var zero V
		c.val = zero
		c.err = nil
		g.pool.Put(c)
	}

	if panicked {
		panic(c.panicErr)
	}

	return val, err, shared
}

func (g *Group[K, V]) doCall(
	c *call[V],
	key K,
	fn func(context.Context) (V, error),
	ctx context.Context,
) {
	defer func() {
		if r := recover(); r != nil {
			c.panicErr = &panicError{value: r, stack: debug.Stack()}
		}

		g.mu.Lock()
		if !c.forgotten {
			delete(g.calls, key)
		}
		// 在锁内捕获 shared 状态，
		// 防止 Leader 返回路径无锁读 dups 产生 data race。
		c.shared = c.dups > 0
		done := c.done
		g.mu.Unlock()

		// 唤醒大量 Follower 会触发调度器，必须放在锁外。
		if done != nil {
			close(done)
		}
		c.wg.Done()
	}()

	c.val, c.err = fn(ctx)
}

// Forget 使 Group 忘记指定 key。
// 下一次对该 key 的 Do 调用将执行 fn 而非等待先前的调用。
func (g *Group[K, V]) Forget(key K) {
	g.mu.Lock()
	if c, ok := g.calls[key]; ok {
		c.forgotten = true
		delete(g.calls, key)
	}
	g.mu.Unlock()
}

// panicError 包装 panic 值和调用栈，
// 使 Follower 收到的 panic 包含原始现场信息而非二次 panic 的栈。
type panicError struct {
	value any
	stack []byte
}

func (p *panicError) Error() string {
	return fmt.Sprintf("%v\n\n%s", p.value, p.stack)
}

// Unwrap 允许 errors.Is / errors.As 穿透到原始 error。
func (p *panicError) Unwrap() error {
	err, ok := p.value.(error)
	if !ok {
		return nil
	}
	return err
}
