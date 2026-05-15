// Package concurrent gives Nguyen.go applications first-class access to
// Go's CSP primitives — bounded worker pools, racing sources, parallel
// fan-out and graceful lifecycle — wrapped in typed APIs that respect
// context cancellation.
//
// These helpers exist because every JavaScript framework has to bolt
// concurrency on top of a single event loop. Go has goroutines and
// channels in the runtime; this package only exposes them with sane
// defaults so handlers, loaders and Spores can use them directly.
package concurrent

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// ErrPoolClosed is returned by Pool.Submit after Close has been called.
var ErrPoolClosed = errors.New("concurrent: pool closed")

// ErrAllSourcesFailed is returned by Race when every source returned an error
// (or the context was cancelled before any source produced a value).
var ErrAllSourcesFailed = errors.New("concurrent: all sources failed")

// Pool is a typed, bounded worker pool that consumes jobs of type J and
// emits results of type R. Worker count defaults to runtime.NumCPU().
//
// The pool is goroutine-safe and respects ctx cancellation: closing the
// pool drains the in-flight jobs but rejects new submissions. Callers
// should range over Results() until it closes to observe completion.
type Pool[J any, R any] struct {
	jobs    chan job[J, R]
	workers int
	handle  func(context.Context, J) (R, error)
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	closed  chan struct{}
	once    sync.Once
}

type job[J any, R any] struct {
	value J
	reply chan Result[R]
}

// Result is the outcome of a single job run by a Pool.
type Result[R any] struct {
	Value R
	Err   error
}

// NewPool builds a Pool with `workers` goroutines (clamped to >= 1).
// `handle` is called once per submitted job.
func NewPool[J any, R any](ctx context.Context, workers int, handle func(context.Context, J) (R, error)) *Pool[J, R] {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if handle == nil {
		panic("concurrent: NewPool requires a non-nil handler")
	}
	c, cancel := context.WithCancel(ctx)
	p := &Pool[J, R]{
		jobs:    make(chan job[J, R], workers*2),
		workers: workers,
		handle:  handle,
		ctx:     c,
		cancel:  cancel,
		closed:  make(chan struct{}),
	}
	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.run()
	}
	return p
}

func (p *Pool[J, R]) run() {
	defer p.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case j, ok := <-p.jobs:
			if !ok {
				return
			}
			val, err := safeCall(p.ctx, p.handle, j.value)
			j.reply <- Result[R]{Value: val, Err: err}
			close(j.reply)
		}
	}
}

func safeCall[J any, R any](ctx context.Context, fn func(context.Context, J) (R, error), in J) (out R, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("concurrent: panic in pool handler: %v", r)
		}
	}()
	return fn(ctx, in)
}

// Submit enqueues one job and returns a result channel that closes after
// the job completes. The returned channel always emits exactly one value
// unless the pool is closed before the job runs.
func (p *Pool[J, R]) Submit(in J) (<-chan Result[R], error) {
	select {
	case <-p.closed:
		return nil, ErrPoolClosed
	default:
	}
	reply := make(chan Result[R], 1)
	select {
	case <-p.closed:
		return nil, ErrPoolClosed
	case <-p.ctx.Done():
		return nil, p.ctx.Err()
	case p.jobs <- job[J, R]{value: in, reply: reply}:
		return reply, nil
	}
}

// SubmitAndWait runs Submit and blocks until the result is ready, ctx is
// done, or the pool is closed.
func (p *Pool[J, R]) SubmitAndWait(ctx context.Context, in J) (R, error) {
	var zero R
	ch, err := p.Submit(in)
	if err != nil {
		return zero, err
	}
	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case r := <-ch:
		return r.Value, r.Err
	}
}

// Close prevents new submissions and waits for in-flight jobs to finish.
// It is safe to call more than once; later calls are no-ops.
func (p *Pool[J, R]) Close() {
	p.once.Do(func() {
		close(p.closed)
		close(p.jobs)
		p.wg.Wait()
		p.cancel()
	})
}

// Map runs `fn` once per input across `workers` goroutines and returns
// the results in the same order as `inputs`. The first error returned by
// any worker cancels the rest and is propagated to the caller.
func Map[I any, O any](ctx context.Context, workers int, inputs []I, fn func(context.Context, I) (O, error)) ([]O, error) {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	out := make([]O, len(inputs))
	if len(inputs) == 0 {
		return out, nil
	}

	c, cancel := context.WithCancel(ctx)
	defer cancel()

	type indexed struct {
		i int
	}
	jobs := make(chan indexed)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				select {
				case <-c.Done():
					return
				default:
				}
				v, err := fn(c, inputs[j.i])
				if err != nil {
					select {
					case errCh <- err:
						cancel()
					default:
					}
					return
				}
				out[j.i] = v
			}
		}()
	}

	for i := range inputs {
		select {
		case <-c.Done():
			break
		case jobs <- indexed{i: i}:
		}
	}
	close(jobs)
	wg.Wait()

	select {
	case err := <-errCh:
		return nil, err
	default:
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// Parallel runs all functions concurrently and returns the first error
// encountered, or nil. ctx cancellation propagates to the inner functions.
//
// This is a fan-out helper that matches the most common Nguyen.go loader
// pattern — fetch user, fetch posts, fetch comments at the same time.
func Parallel(ctx context.Context, fns ...func(context.Context) error) error {
	if len(fns) == 0 {
		return nil
	}
	c, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, len(fns))
	var wg sync.WaitGroup
	wg.Add(len(fns))
	for _, fn := range fns {
		fn := fn
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					errCh <- fmt.Errorf("concurrent: parallel panic: %v", r)
					cancel()
				}
			}()
			if err := fn(c); err != nil {
				errCh <- err
				cancel()
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

// RaceSource produces a typed value or an error. Sources are run in
// parallel by Race; the first to return a non-error value wins.
type RaceSource[T any] func(context.Context) (T, error)

// Race starts every source goroutine and returns the first successful
// value. If all sources return errors (or ctx is cancelled before any
// success) the returned error wraps ErrAllSourcesFailed and includes
// every source error.
//
// Common pattern: race a primary source against a cached fallback with
// a small timeout source so callers always get an answer fast.
func Race[T any](ctx context.Context, sources ...RaceSource[T]) (T, error) {
	var zero T
	if len(sources) == 0 {
		return zero, ErrAllSourcesFailed
	}

	c, cancel := context.WithCancel(ctx)
	defer cancel()

	winner := make(chan T, 1)
	errs := make(chan error, len(sources))

	for _, s := range sources {
		s := s
		go func() {
			defer func() {
				if r := recover(); r != nil {
					errs <- fmt.Errorf("concurrent: race panic: %v", r)
				}
			}()
			v, err := s(c)
			if err != nil {
				errs <- err
				return
			}
			select {
			case winner <- v:
				cancel()
			default:
			}
		}()
	}

	collected := 0
	var collectedErrs []error
	for {
		select {
		case v := <-winner:
			return v, nil
		case err := <-errs:
			collected++
			collectedErrs = append(collectedErrs, err)
			if collected >= len(sources) {
				return zero, fmt.Errorf("%w: %v", ErrAllSourcesFailed, collectedErrs)
			}
		case <-ctx.Done():
			return zero, ctx.Err()
		}
	}
}

// Deadline runs `fn` with a derived context that expires after `d`.
// If the function does not respect ctx, Deadline still returns when the
// deadline elapses but the goroutine running fn may continue — this is a
// limitation of cooperative cancellation.
func Deadline[T any](ctx context.Context, d time.Duration, fn func(context.Context) (T, error)) (T, error) {
	c, cancel := context.WithTimeout(ctx, d)
	defer cancel()

	type result struct {
		v   T
		err error
	}
	done := make(chan result, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				var zero T
				done <- result{v: zero, err: fmt.Errorf("concurrent: deadline panic: %v", r)}
			}
		}()
		v, err := fn(c)
		done <- result{v: v, err: err}
	}()

	select {
	case r := <-done:
		return r.v, r.err
	case <-c.Done():
		var zero T
		return zero, c.Err()
	}
}
