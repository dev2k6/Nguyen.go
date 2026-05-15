package concurrent

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestLifecycleStartsAndStops(t *testing.T) {
	lc := NewLifecycle(time.Second)

	var started, stopped int32
	lc.Add("a",
		func(c context.Context) error {
			atomic.AddInt32(&started, 1)
			<-c.Done()
			return nil
		},
		func(_ context.Context) error {
			atomic.AddInt32(&stopped, 1)
			return nil
		},
	)
	lc.Add("b",
		func(c context.Context) error {
			atomic.AddInt32(&started, 1)
			<-c.Done()
			return nil
		},
		func(_ context.Context) error {
			atomic.AddInt32(&stopped, 1)
			return nil
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(40 * time.Millisecond)
		cancel()
	}()

	if err := lc.Run(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if atomic.LoadInt32(&started) != 2 {
		t.Fatalf("want 2 starts got %d", started)
	}
	if atomic.LoadInt32(&stopped) != 2 {
		t.Fatalf("want 2 stops got %d", stopped)
	}
}

func TestLifecycleStartErrorTriggersShutdown(t *testing.T) {
	lc := NewLifecycle(time.Second)

	want := errors.New("boom")
	var stoppedB int32
	lc.Add("a",
		func(_ context.Context) error { return want },
		func(_ context.Context) error { return nil },
	)
	lc.Add("b",
		func(c context.Context) error { <-c.Done(); return nil },
		func(_ context.Context) error { atomic.AddInt32(&stoppedB, 1); return nil },
	)

	err := lc.Run(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, want) {
		t.Fatalf("want wrapped boom got %v", err)
	}
	if atomic.LoadInt32(&stoppedB) != 1 {
		t.Fatalf("want b stopped once got %d", stoppedB)
	}
}

func TestLifecycleStopRunsInReverseOrder(t *testing.T) {
	lc := NewLifecycle(time.Second)

	var order []string
	var mu = make(chan struct{}, 1)
	mu <- struct{}{}

	push := func(s string) {
		<-mu
		order = append(order, s)
		mu <- struct{}{}
	}

	for _, name := range []string{"a", "b", "c"} {
		n := name
		lc.Add(n,
			func(c context.Context) error { <-c.Done(); return nil },
			func(_ context.Context) error { push(n); return nil },
		)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	_ = lc.Run(ctx)

	want := []string{"c", "b", "a"}
	if len(order) != len(want) {
		t.Fatalf("want %v got %v", want, order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("at %d want %s got %s", i, want[i], order[i])
		}
	}
}

func TestLifecycleAddAfterRunPanics(t *testing.T) {
	lc := NewLifecycle(time.Second)
	lc.Add("a",
		func(c context.Context) error { <-c.Done(); return nil },
		nil,
	)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(10 * time.Millisecond); cancel() }()
	go func() { _ = lc.Run(ctx) }()
	time.Sleep(5 * time.Millisecond)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	lc.Add("b", nil, nil)
}
