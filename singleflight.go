package singleflight

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
)

// Group represents a class of work and forms a namespace in
// which units of work can be executed with duplicate suppression.
//
// It provides a generic, type-safe, and zero-allocation (in steady state)
// implementation of the singleflight pattern.
type Group[K comparable, V any] struct {
	calls map[K]*call[V]
	mu    sync.Mutex
	pool  sync.Pool
}

// call stores information about a single function call.
type call[V any] struct {
	val V
	err error

	// panicErr holds the panic value if the function panicked.
	panicErr *panicError

	// done is lazily initialized. It remains nil for non-shared calls.
	done chan struct{}

	// dups counts the number of waiters.
	dups int

	// forgotten indicates if Forget was called.
	forgotten bool
}

// NewGroup creates a new Group.
func NewGroup[K comparable, V any]() *Group[K, V] {
	return &Group[K, V]{
		calls: make(map[K]*call[V]),
		pool: sync.Pool{
			New: func() any {
				return new(call[V])
			},
		},
	}
}

// Do executes and returns the results of the given function, making
// sure that only one execution is in-flight for a given key at a
// time.
func (g *Group[K, V]) Do(ctx context.Context, key K, fn func(ctx context.Context) (V, error)) (v V, shared bool, err error) {
	g.mu.Lock()
	if g.calls == nil {
		g.calls = make(map[K]*call[V])
	}

	// 1. Join existing call (Follower)
	if c, ok := g.calls[key]; ok {
		c.dups++
		// Lazy Init Channel for Followers
		if c.done == nil {
			c.done = make(chan struct{})
		}
		done := c.done
		g.mu.Unlock()

		select {
		case <-done:
			if c.panicErr != nil {
				panic(c.panicErr)
			}
			return c.val, true, c.err
		case <-ctx.Done():
			return *new(V), true, ctx.Err()
		}
	}

	// 2. Start new call (Leader)
	var c *call[V]
	if val := g.pool.Get(); val != nil {
		c = val.(*call[V])
	} else {
		c = new(call[V])
	}

	// Reset state
	c.dups = 0
	c.forgotten = false
	c.panicErr = nil
	// c.done is guaranteed to be nil here from recycling logic

	g.calls[key] = c
	g.mu.Unlock()

	// Execute Synchronously
	g.doCall(c, key, fn, ctx)

	// 3. Leader Return & Recycle logic
	val := c.val
	err = c.err
	panicked := c.panicErr != nil

	// We can ONLY recycle if:
	// 1. No panic occurred (safety first).
	// 2. No followers joined (dups == 0).
	// 3. No channel was created (done == nil).
	if !panicked && c.dups == 0 && c.done == nil {
		// Zero out fields to prevent memory leaks
		var zero V
		c.val = zero
		c.err = nil
		g.pool.Put(c)
	}

	if panicked {
		panic(c.panicErr)
	}

	return val, c.dups > 0, err
}

// doCall handles the execution of the user function.
func (g *Group[K, V]) doCall(c *call[V], key K, fn func(context.Context) (V, error), ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			c.panicErr = &panicError{value: r, stack: debug.Stack()}
		}

		g.mu.Lock()
		if c.done != nil {
			close(c.done)
		}
		if !c.forgotten {
			delete(g.calls, key)
		}
		g.mu.Unlock()
	}()

	c.val, c.err = fn(ctx)
}

// Forget tells the singleflight to forget about a key.
func (g *Group[K, V]) Forget(key K) {
	g.mu.Lock()
	if c, ok := g.calls[key]; ok {
		c.forgotten = true
	}
	delete(g.calls, key)
	g.mu.Unlock()
}

// panicError wraps a panic value and its stack trace.
type panicError struct {
	value any
	stack []byte
}

func (p *panicError) Error() string {
	return fmt.Sprintf("%v\n\n%s", p.value, p.stack)
}

func (p *panicError) Unwrap() error {
	err, ok := p.value.(error)
	if !ok {
		return nil
	}
	return err
}
