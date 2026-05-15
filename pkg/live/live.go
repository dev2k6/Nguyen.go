// Package live re-exports the public surface of internal/live for
// application code. Live Mode pages typically only need the Event type
// (for typed event names) — the Hub and Session machinery is owned by
// the framework itself.
//
// Keeping the public API tiny lets us evolve the wire protocol and
// session internals without breaking pages.
package live

import internalLive "github.com/dev2k6/Nguyen.go/internal/live"

// Event is one inbound message from a Live Mode client. It is a type
// alias of internal/live.Event so application code can implement
// livepage.Page without depending on internal packages.
type Event = internalLive.Event
