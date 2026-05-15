// Package live implements server-pushed VDOM diffing over WebSocket.
//
// In Live Mode, an entire .gox page is rendered on the server, the
// browser opens a WebSocket back to the same route, and every user
// interaction is shipped as a small JSON event. Server handlers mutate
// session state, the renderer produces a new tree, the differ emits a
// minimal patch list, and the bridge applies the patch in-place.
//
// The result is interactive UI without a WASM bundle: pages can ship
// 0 KB of compiled Go to the browser and still respond to clicks,
// inputs and form submissions. The cost is one goroutine + a few
// kilobytes of state per active connection — affordable on Go because
// goroutines are cheap and context cancellation is free.
//
// This file holds the type skeleton (Session, Hub, options). The diff
// algorithm and protocol are in diff.go and protocol.go respectively.
package live

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrSessionNotFound is returned by Hub.Get when no session matches an id.
var ErrSessionNotFound = errors.New("live: session not found")

// ErrTooManySessions is returned by Hub.Spawn when the configured limit
// is reached. Tune via Options.MaxSessions.
var ErrTooManySessions = errors.New("live: too many sessions")

// Options controls a Hub's runtime behaviour.
type Options struct {
	// MaxSessions is the soft cap on simultaneous live sessions. Spawn
	// returns ErrTooManySessions when reached. Default: 10000.
	MaxSessions int

	// IdleTimeout closes a session that has had no inbound event for
	// the given duration. Default: 60s.
	IdleTimeout time.Duration

	// HeartbeatInterval is how often the server sends a ping frame
	// when no other traffic is flowing. Default: 30s.
	HeartbeatInterval time.Duration

	// OutboundBuffer is the per-session capacity of the channel used
	// to queue server→client messages. Default: 16.
	OutboundBuffer int
}

func (o Options) withDefaults() Options {
	if o.MaxSessions <= 0 {
		o.MaxSessions = 10000
	}
	if o.IdleTimeout <= 0 {
		o.IdleTimeout = 60 * time.Second
	}
	if o.HeartbeatInterval <= 0 {
		o.HeartbeatInterval = 30 * time.Second
	}
	if o.OutboundBuffer <= 0 {
		o.OutboundBuffer = 16
	}
	return o
}

// Renderer turns the current state into a server-rendered HTML
// fragment. It is called once per pending update; the differ takes
// the previous and next outputs and emits a patch list.
//
// Implementations should be deterministic — the same state must yield
// the same HTML — so the diff is meaningful.
type Renderer interface {
	Render(ctx context.Context, state any) (string, error)
}

// Handler dispatches an inbound event to the page's Go-defined methods
// and returns the new state. It must not block on I/O without a ctx
// check; long work belongs in a goroutine spawned via the session's
// Spawn helper so it can be cancelled when the session closes.
type Handler interface {
	Handle(ctx context.Context, state any, evt Event) (any, error)
}

// Session is one live WebSocket connection bound to a Renderer +
// Handler pair. State is private to the session and survives across
// events but not across reconnects (v1.2 is in-memory only).
type Session struct {
	id        string
	route     string
	state     any
	renderer  Renderer
	handler   Handler
	lastHTML  string
	outbound  chan Outbound
	closed    chan struct{}
	closeOnce sync.Once
	ctx       context.Context
	cancel    context.CancelFunc
	lastSeen  time.Time
	mu        sync.Mutex
	opts      Options
}

// ID returns the session's stable identifier.
func (s *Session) ID() string { return s.id }

// Route returns the route pattern this session is bound to.
func (s *Session) Route() string { return s.route }

// Context returns the session's context. It is cancelled when Close
// is called or the Hub shuts down.
func (s *Session) Context() context.Context { return s.ctx }

// Outbound is one message queued for the client. The protocol layer
// turns it into JSON.
type Outbound struct {
	Kind string // "diff", "replace", "ping", "error"
	Diff []Op
	HTML string
	Err  string
}

// Outbound returns the channel of messages waiting to be flushed to
// the client. The transport (WebSocket handler) drains it.
func (s *Session) OutboundCh() <-chan Outbound { return s.outbound }

// Closed returns a channel that is closed once Close has run. Wait on
// it from the transport's read loop.
func (s *Session) Closed() <-chan struct{} { return s.closed }

// Close terminates the session, cancels its context and notifies the
// hub. Safe to call repeatedly.
func (s *Session) Close() {
	s.closeOnce.Do(func() {
		s.cancel()
		close(s.closed)
	})
}

// touch updates the last-seen timestamp. The hub's idle reaper checks
// this against IdleTimeout.
func (s *Session) touch() {
	s.mu.Lock()
	s.lastSeen = time.Now()
	s.mu.Unlock()
}

// Dispatch processes one inbound event end-to-end: handler → state
// transition → render → diff → enqueue outbound. Errors during render
// or handle are surfaced as an "error" outbound message; the session
// stays open so the client can recover.
//
// Dispatch is not concurrent-safe — call it from a single goroutine
// (the readLoop). The mu lock protects state so that touch() and
// concurrent reads from other goroutines see a consistent value.
func (s *Session) Dispatch(evt Event) {
	s.touch()
	s.mu.Lock()
	newState, err := s.handler.Handle(s.ctx, s.state, evt)
	if err != nil {
		s.mu.Unlock()
		s.send(Outbound{Kind: "error", Err: err.Error()})
		return
	}
	s.state = newState
	s.mu.Unlock()

	html, err := s.renderer.Render(s.ctx, s.state)
	if err != nil {
		s.send(Outbound{Kind: "error", Err: err.Error()})
		return
	}

	s.mu.Lock()
	prev := s.lastHTML
	s.lastHTML = html
	s.mu.Unlock()

	if prev == "" {
		s.send(Outbound{Kind: "replace", HTML: html})
		return
	}
	ops := Diff(prev, html)
	if len(ops) == 0 {
		return
	}
	s.send(Outbound{Kind: "diff", Diff: ops})
}

// send pushes a message onto the outbound channel. If the buffer is
// full the message is dropped — the client will fall behind and the
// transport may decide to disconnect. Recording drops here is the
// responsibility of the transport which observes channel pressure.
func (s *Session) send(o Outbound) {
	select {
	case s.outbound <- o:
	default:
		// Drop on overflow; transport sees an idle gap and may close
		// the session. This is preferred over blocking the dispatcher.
	}
}

// Hub owns all live sessions for a process. It hands out new sessions,
// looks them up by id, enforces MaxSessions, and reaps idle ones.
type Hub struct {
	opts     Options
	mu       sync.RWMutex
	sessions map[string]*Session
	rootCtx  context.Context
	cancel   context.CancelFunc
}

// NewHub returns a Hub ready for Spawn. Call Run from a Lifecycle to
// start the idle reaper.
func NewHub(opts Options) *Hub {
	opts = opts.withDefaults()
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		opts:     opts,
		sessions: make(map[string]*Session),
		rootCtx:  ctx,
		cancel:   cancel,
	}
}

// Run blocks until ctx is cancelled OR Stop is called, periodically
// reaping sessions that have not received an event within IdleTimeout.
// Wire it through pkg/concurrent.Lifecycle.
//
// The external ctx and the hub's internal rootCtx are both respected so
// that either a Lifecycle shutdown or a direct Stop() call terminates
// the reaper. Sessions spawned after Stop is called will have their
// context cancelled immediately via rootCtx.
func (h *Hub) Run(ctx context.Context) error {
	ticker := time.NewTicker(h.opts.IdleTimeout / 2)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			// External context cancelled — stop the reaper but do NOT
			// close sessions; the caller is responsible for calling Stop
			// if it wants sessions cleaned up.
			return nil
		case <-h.rootCtx.Done():
			return nil
		case now := <-ticker.C:
			h.reap(now)
		}
	}
}

// Stop cancels every session. It is the Lifecycle stop hook.
func (h *Hub) Stop(ctx context.Context) error {
	h.cancel()
	h.mu.Lock()
	for _, s := range h.sessions {
		s.Close()
	}
	h.sessions = make(map[string]*Session)
	h.mu.Unlock()
	return nil
}

// Count returns the number of currently-live sessions.
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.sessions)
}

// Spawn creates a new session bound to the given route, renderer and
// initial state. The id must be globally unique within the hub —
// callers usually pass a 16-byte hex string from ngctx.NewID.
// Returns ErrTooManySessions when the cap is reached, or an error if
// the hub has already been stopped (rootCtx cancelled).
func (h *Hub) Spawn(parentCtx context.Context, id, route string, initialState any, renderer Renderer, handler Handler) (*Session, error) {
	if renderer == nil || handler == nil {
		return nil, errors.New("live: renderer and handler are required")
	}
	h.mu.Lock()
	// Check rootCtx under the lock to avoid a race with Stop().
	if h.rootCtx.Err() != nil {
		h.mu.Unlock()
		return nil, errors.New("live: hub is stopped")
	}
	if len(h.sessions) >= h.opts.MaxSessions {
		h.mu.Unlock()
		return nil, ErrTooManySessions
	}
	if _, exists := h.sessions[id]; exists {
		h.mu.Unlock()
		return nil, errors.New("live: session id already in use")
	}
	ctx, cancel := context.WithCancel(parentCtx)
	s := &Session{
		id:       id,
		route:    route,
		state:    initialState,
		renderer: renderer,
		handler:  handler,
		outbound: make(chan Outbound, h.opts.OutboundBuffer),
		closed:   make(chan struct{}),
		ctx:      ctx,
		cancel:   cancel,
		lastSeen: time.Now(),
		opts:     h.opts,
	}
	h.sessions[id] = s
	h.mu.Unlock()

	go func() {
		<-s.closed
		h.mu.Lock()
		delete(h.sessions, id)
		h.mu.Unlock()
	}()
	return s, nil
}

// Get returns the session with the given id or ErrSessionNotFound.
func (h *Hub) Get(id string) (*Session, error) {
	h.mu.RLock()
	s, ok := h.sessions[id]
	h.mu.RUnlock()
	if !ok {
		return nil, ErrSessionNotFound
	}
	return s, nil
}

func (h *Hub) reap(now time.Time) {
	deadline := now.Add(-h.opts.IdleTimeout)
	h.mu.RLock()
	stale := make([]*Session, 0)
	for _, s := range h.sessions {
		s.mu.Lock()
		seen := s.lastSeen
		s.mu.Unlock()
		if seen.Before(deadline) {
			stale = append(stale, s)
		}
	}
	h.mu.RUnlock()
	for _, s := range stale {
		s.Close()
	}
}
