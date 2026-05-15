package live

import (
	"encoding/json"
	"errors"
)

// Event is one inbound message from the client. Kind is always
// "event" today; future kinds (form, file, navigation) will share
// the same envelope.
type Event struct {
	Kind   string          `json:"t"`
	Name   string          `json:"name,omitempty"`
	Target string          `json:"target,omitempty"`
	Value  json.RawMessage `json:"value,omitempty"`
	Data   json.RawMessage `json:"data,omitempty"`
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
	return e, nil
}

// EncodeOutbound turns a queued message into one server→client JSON
// frame.
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
	default:
		return nil, errors.New("live: unknown outbound kind " + o.Kind)
	}
}
