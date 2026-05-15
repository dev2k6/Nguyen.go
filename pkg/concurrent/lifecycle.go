package concurrent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Lifecycle coordinates ordered start/stop of long-lived components such
// as HTTP servers, WebSocket hubs, DB pools, cron schedulers and message
// brokers. Components register Start and Stop functions; Run blocks until
// the supplied context is cancelled and then runs Stop in reverse order.
//
// This is the small piece you usually wire by hand with errgroup +
// signal.NotifyContext + a slice of "shutters". Lifecycle wraps the
// pattern so applications get deterministic shutdown for free.
type Lifecycle struct {
	mu         sync.Mutex
	components []component
	stopGrace  time.Duration
	started    bool
}

type component struct {
	name  string
	start func(context.Context) error
	stop  func(context.Context) error
}

// NewLifecycle creates an empty Lifecycle. stopGrace bounds the time
// allowed for any single Stop call; pass 0 to disable the limit.
func NewLifecycle(stopGrace time.Duration) *Lifecycle {
	return &Lifecycle{stopGrace: stopGrace}
}

// Add registers a component. Components run Start in the order they
// were added and Stop in reverse order. name is used in error messages.
//
// start may block (e.g. http.Server.ListenAndServe). It is executed in
// its own goroutine and any non-nil error it returns triggers shutdown.
// stop is called once during shutdown with a context bounded by
// stopGrace.
func (l *Lifecycle) Add(name string, start, stop func(context.Context) error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.started {
		panic("concurrent: Lifecycle.Add called after Run")
	}
	l.components = append(l.components, component{name: name, start: start, stop: stop})
}

// Run starts all components and blocks until ctx is cancelled or any
// component's Start returns. It then runs Stop on every component in
// reverse order and returns the joined error, if any.
func (l *Lifecycle) Run(ctx context.Context) error {
	l.mu.Lock()
	if l.started {
		l.mu.Unlock()
		return errors.New("concurrent: Lifecycle already running")
	}
	l.started = true
	comps := make([]component, len(l.components))
	copy(comps, l.components)
	l.mu.Unlock()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	startErrs := make(chan namedErr, len(comps))
	var wg sync.WaitGroup
	for _, c := range comps {
		c := c
		if c.start == nil {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					startErrs <- namedErr{name: c.name, err: fmt.Errorf("panic: %v", r)}
					cancel()
				}
			}()
			if err := c.start(runCtx); err != nil && !errors.Is(err, context.Canceled) {
				startErrs <- namedErr{name: c.name, err: err}
				cancel()
			}
		}()
	}

	<-runCtx.Done()

	stopErrs := l.stopAll(comps)
	wg.Wait()
	close(startErrs)

	var allErrs []error
	for e := range startErrs {
		allErrs = append(allErrs, fmt.Errorf("%s: %w", e.name, e.err))
	}
	allErrs = append(allErrs, stopErrs...)
	if len(allErrs) == 0 {
		return nil
	}
	return errors.Join(allErrs...)
}

func (l *Lifecycle) stopAll(comps []component) []error {
	var errs []error
	for i := len(comps) - 1; i >= 0; i-- {
		c := comps[i]
		if c.stop == nil {
			continue
		}
		ctx := context.Background()
		var cancel context.CancelFunc
		if l.stopGrace > 0 {
			ctx, cancel = context.WithTimeout(ctx, l.stopGrace)
		}
		err := func() (e error) {
			defer func() {
				if r := recover(); r != nil {
					e = fmt.Errorf("panic: %v", r)
				}
			}()
			return c.stop(ctx)
		}()
		if cancel != nil {
			cancel()
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", c.name, err))
		}
	}
	return errs
}

type namedErr struct {
	name string
	err  error
}
