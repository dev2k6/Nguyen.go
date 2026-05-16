package live

import (
	"encoding/json"
	"errors"
)

// Event is one inbound message from the client.
//
// Kind values:
//   - "event"    — a named user interaction (click, input, etc.)
//   - "form"     — a form submission; FormData carries all field values
//   - "navigate" — client-side navigation; Path is the target URL
//   - "reattach" — reconnect with an existing session token
type Event struct {
	Kind     string            `json:"t"`
	Name     string            `json:"name,omitempty"`
	Target   string            `json:"target,omitempty"`
	Value    json.RawMessage   `json:"value,omitempty"`
	Data     json.RawMessage   `json:"data,omitempty"`
	FormData map[string]string `json:"form,omitempty"`
	Path     string            `json:"path,omitempty"`
	Token    string            `json:"token,omitempty"`
}

var validEventKinds = map[string]bool{
	"event":    true,
	"form":     true,
	"navigate": true,
	"reattach": true,
}

// DecodeEvent parses one client→server JSON frame.
func DecodeEvent(buf []byte) (Event, error) {
	var e Event
	if err := json.Unmarshal(buf, &e); err != nil {
		return Event{}, err
	}
	if e.Kind == "" {
		return Event{}, errors.New("live: event missing 't' field")
	}
	if !validEventKinds[e.Kind] {
		return Event{}, errors.New("live: unknown event kind: " + e.Kind)
	}
	return e, nil
}

// EncodeOutbound turns a queued message into one server→client JSON frame.
func EncodeOutbound(o Outbound) ([]byte, error) {
	switch o.Kind {
	case "diff":
		return json.Marshal(struct {
			Kind string `json:"t"`
			Ops  []Op   `json:"ops"`
		}{"diff", o.Diff})
	case "replace":
		return json.Marshal(struct {
			Kind string `json:"t"`
			HTML string `json:"html"`
		}{"replace", o.HTML})
	case "ping":
		return json.Marshal(struct {
			Kind string `json:"t"`
		}{"ping"})
	case "error":
		return json.Marshal(struct {
			Kind string `json:"t"`
			Err  string `json:"error"`
		}{"error", o.Err})
	case "session":
		return json.Marshal(struct {
			Kind  string `json:"t"`
			Token string `json:"token"`
		}{"session", o.Token})
	default:
		return nil, errors.New("live: unknown outbound kind " + o.Kind)
	}
}
