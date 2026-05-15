// Package livedemo defines a Live Mode counter page used by the
// hello-nguyen example. It illustrates the minimum surface area: a
// state struct, an Init/Render/Handle implementation, and a typed
// event dispatch.
package livedemo

import (
	"context"
	"fmt"

	"github.com/dev2k6/Nguyen.go/pkg/live"
)

// CounterState is the per-session state kept on the server.
type CounterState struct {
	Count int
}

// CounterPage is the live page implementation. It is registered at
// /counter and reachable from the client via /_nguyen/live/counter.
type CounterPage struct{}

// Init seeds a fresh counter for every new WebSocket connection.
func (CounterPage) Init(_ context.Context, _ map[string]string) (any, error) {
	return &CounterState{Count: 0}, nil
}

// Render emits the HTML fragment that lives inside the
// <div data-nguyen-live="/counter"> mount. The differ takes care of
// the rest.
func (CounterPage) Render(_ context.Context, state any) (string, error) {
	s := state.(*CounterState)
	return fmt.Sprintf(`
<section style="font-family: system-ui; padding: 2rem; text-align: center">
  <h1 style="font-size: 3rem; margin: 0">Count: %d</h1>
  <p style="color: #6b7280; margin: 0.5rem 0 1.5rem">No WASM. No client state. Pure server-pushed UI.</p>
  <button @click="dec" style="font-size: 1.5rem; padding: 0.5rem 1.25rem; margin-right: 0.5rem">-</button>
  <button @click="inc" style="font-size: 1.5rem; padding: 0.5rem 1.25rem">+</button>
  <button @click="reset" style="font-size: 1rem; padding: 0.5rem 1rem; margin-left: 0.5rem">reset</button>
</section>`, s.Count), nil
}

// Handle dispatches one client event to the state machine.
func (CounterPage) Handle(_ context.Context, state any, evt live.Event) (any, error) {
	s := state.(*CounterState)
	switch evt.Name {
	case "inc":
		s.Count++
	case "dec":
		s.Count--
	case "reset":
		s.Count = 0
	}
	return s, nil
}
