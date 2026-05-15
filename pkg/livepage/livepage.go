// Package livepage exposes the application-facing contract for
// Nguyen.go Live Mode pages. A Page is a struct that implements three
// methods — Init, Render and Handle — and is registered with an App
// via app.RegisterLivePage(pattern, page).
//
// The framework wraps each Page in an internal/live session and
// routes incoming WebSocket events to its Handle method, then ships
// the diff between two Render outputs back to the browser.
package livepage

import (
	"context"

	"github.com/dev2k6/Nguyen.go/pkg/live"
)

// Page is the contract a live route must satisfy. State carries the
// per-session struct, Init seeds it from request params, Render emits
// HTML, and Handle reacts to one client event.
//
// Render must be deterministic for a given state value so the diff
// algorithm produces minimal patches. Handle should mutate the state
// and return it; returning a brand new value is also valid.
type Page interface {
	Init(ctx context.Context, params map[string]string) (state any, err error)
	Render(ctx context.Context, state any) (string, error)
	Handle(ctx context.Context, state any, evt live.Event) (any, error)
}
