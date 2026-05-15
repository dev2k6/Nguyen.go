// Package livepage bridges Live Mode page definitions to internal/live
// at runtime. Application code registers a pkg/livepage.Page per route
// pattern; the WebSocket handler looks the page up by route and
// constructs a Session against it.
package livepage

import (
	"context"
	"errors"
	"sync"

	"github.com/dev2k6/Nguyen.go/internal/live"
	publiclive "github.com/dev2k6/Nguyen.go/pkg/live"
	"github.com/dev2k6/Nguyen.go/pkg/livepage"
)

// Page is an alias of the public livepage.Page so internal callers do
// not need to import both packages.
type Page = livepage.Page

// Registry maps route patterns to Pages. A Hub may serve any number of
// routes by sharing one Registry.
type Registry struct {
	mu    sync.RWMutex
	pages map[string]Page
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry { return &Registry{pages: make(map[string]Page)} }

// Register binds page to route. Re-registering an existing route
// returns an error so duplicate wiring fails loudly during startup.
func (r *Registry) Register(route string, page Page) error {
	if route == "" {
		return errors.New("livepage: route is required")
	}
	if page == nil {
		return errors.New("livepage: page is nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.pages[route]; exists {
		return errors.New("livepage: route " + route + " already registered")
	}
	r.pages[route] = page
	return nil
}

// Get returns the page registered for route, or nil.
func (r *Registry) Get(route string) Page {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.pages[route]
}

// Routes lists every registered route. Order is not guaranteed.
func (r *Registry) Routes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.pages))
	for k := range r.pages {
		out = append(out, k)
	}
	return out
}

// Adapter exposes a Page as the Renderer + Handler pair internal/live
// expects. Callers usually do not invoke this directly — the WebSocket
// transport calls it during Spawn.
type Adapter struct{ Page Page }

// Render satisfies live.Renderer.
func (a Adapter) Render(ctx context.Context, state any) (string, error) {
	return a.Page.Render(ctx, state)
}

// Handle satisfies live.Handler. It accepts the internal/live event
// type and forwards a publicly-typed event to the Page so application
// code never imports internal/live.
func (a Adapter) Handle(ctx context.Context, state any, evt live.Event) (any, error) {
	return a.Page.Handle(ctx, state, publiclive.Event(evt))
}

