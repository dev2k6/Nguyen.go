package concurrent

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestPoolSubmitAndWait(t *testing.T) {
	ctx := context.Background()
	p := NewPool(ctx, 2, func(_ context.Context, n int) (int, error) {
		return n * 2, nil
	})
	defer p.Close()

	got, err := p.SubmitAndWait(ctx, 21)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got != 42 {
		t.Fatalf("want 42 got %d", got)
	}
}

func TestPoolHandlerError(t *testing.T) {
	ctx := context.Background()
	want := errors.New("boom")
	p := NewPool(ctx, 1, func(_ context.Context, n int) (int, error) {
		return 0, want
	})
	defer p.Close()

	_, err := p.SubmitAndWait(ctx, 1)
	if !errors.Is(err, want) {
		t.Fatalf("want %v got %v", want, err)
	}
}

func TestPoolPanicRecovers(t *testing.T) {
	ctx := context.Background()
	p := NewPool(ctx, 1, func(_ context.Context, n int) (int, error) {
		panic("kaboom")
	})
	defer p.Close()

	_, err := p.SubmitAndWait(ctx, 1)
	if err == nil {
		t.Fatal("expected panic to surface as error")
	}
}

func TestPoolClosedRejects(t *testing.T) {
	ctx := context.Background()
	p := NewPool(ctx, 1, func(_ context.Context, n int) (int, error) {
		return n, nil
	})
	p.Close()

	_, err := p.Submit(1)
	if !errors.Is(err, ErrPoolClosed) {
		t.Fatalf("want ErrPoolClosed got %v", err)
	}
}

func TestMapPreservesOrder(t *testing.T) {
	ctx := context.Background()
	in := []int{1, 2, 3, 4, 5}
	out, err := Map(ctx, 3, in, func(_ context.Context, n int) (int, error) {
		return n * n, nil
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	want := []int{1, 4, 9, 16, 25}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("at %d want %d got %d", i, want[i], out[i])
		}
	}
}

func TestMapErrorCancelsOthers(t *testing.T) {
	ctx := context.Background()
	want := errors.New("nope")
	var ran int32
	_, err := Map(ctx, 4, []int{1, 2, 3, 4, 5, 6, 7, 8}, func(c context.Context, n int) (int, error) {
		atomic.AddInt32(&ran, 1)
		if n == 3 {
			return 0, want
		}
		select {
		case <-c.Done():
			return 0, c.Err()
		case <-time.After(20 * time.Millisecond):
			return n, nil
		}
	})
	if !errors.Is(err, want) {
		t.Fatalf("want %v got %v", want, err)
	}
}

func TestParallelAllSucceed(t *testing.T) {
	ctx := context.Background()
	var a, b int32
	err := Parallel(ctx,
		func(_ context.Context) error { atomic.StoreInt32(&a, 1); return nil },
		func(_ context.Context) error { atomic.StoreInt32(&b, 2); return nil },
	)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if a != 1 || b != 2 {
		t.Fatalf("want a=1 b=2 got a=%d b=%d", a, b)
	}
}

func TestParallelFirstErrorWins(t *testing.T) {
	ctx := context.Background()
	want := errors.New("x")
	err := Parallel(ctx,
		func(c context.Context) error {
			select {
			case <-c.Done():
				return c.Err()
			case <-time.After(50 * time.Millisecond):
				return nil
			}
		},
		func(_ context.Context) error { return want },
	)
	if !errors.Is(err, want) {
		t.Fatalf("want %v got %v", want, err)
	}
}

func TestRaceFirstSuccessWins(t *testing.T) {
	ctx := context.Background()
	got, err := Race(ctx,
		func(c context.Context) (string, error) {
			select {
			case <-c.Done():
				return "", c.Err()
			case <-time.After(60 * time.Millisecond):
				return "slow", nil
			}
		},
		func(_ context.Context) (string, error) {
			return "fast", nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got != "fast" {
		t.Fatalf("want fast got %s", got)
	}
}

func TestRaceAllFail(t *testing.T) {
	ctx := context.Background()
	_, err := Race(ctx,
		func(_ context.Context) (int, error) { return 0, errors.New("a") },
		func(_ context.Context) (int, error) { return 0, errors.New("b") },
	)
	if !errors.Is(err, ErrAllSourcesFailed) {
		t.Fatalf("want ErrAllSourcesFailed got %v", err)
	}
}

func TestDeadlineFiresOnTimeout(t *testing.T) {
	ctx := context.Background()
	_, err := Deadline(ctx, 30*time.Millisecond, func(c context.Context) (int, error) {
		<-c.Done()
		return 0, c.Err()
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want DeadlineExceeded got %v", err)
	}
}

func TestDeadlineReturnsValue(t *testing.T) {
	ctx := context.Background()
	got, err := Deadline(ctx, time.Second, func(_ context.Context) (int, error) {
		return 7, nil
	})
	if err != nil || got != 7 {
		t.Fatalf("want 7 nil got %d %v", got, err)
	}
}
