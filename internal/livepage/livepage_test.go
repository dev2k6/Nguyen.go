package livepage

import (
	"context"
	"errors"
	"strconv"
	"testing"

	publiclive "github.com/dev2k6/Nguyen.go/pkg/live"
)

type counterState struct{ Count int }

type counterPage struct{ initial int }

func (c *counterPage) Init(_ context.Context, _ map[string]string) (any, error) {
	return &counterState{Count: c.initial}, nil
}

func (c *counterPage) Render(_ context.Context, state any) (string, error) {
	s := state.(*counterState)
	return "<p>count:" + strconv.Itoa(s.Count) + "</p>", nil
}

func (c *counterPage) Handle(_ context.Context, state any, evt publiclive.Event) (any, error) {
	s := state.(*counterState)
	switch evt.Name {
	case "inc":
		s.Count++
	case "dec":
		s.Count--
	default:
		return nil, errors.New("unknown event")
	}
	return s, nil
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	p := &counterPage{}
	if err := r.Register("/counter", p); err != nil {
		t.Fatal(err)
	}
	if got := r.Get("/counter"); got != p {
		t.Fatalf("want page got %v", got)
	}
	if got := r.Get("/missing"); got != nil {
		t.Fatalf("expected nil got %v", got)
	}
}

func TestRegistryRejectsDuplicate(t *testing.T) {
	r := NewRegistry()
	if err := r.Register("/x", &counterPage{}); err != nil {
		t.Fatal(err)
	}
	if err := r.Register("/x", &counterPage{}); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestRegistryRejectsEmpty(t *testing.T) {
	r := NewRegistry()
	if err := r.Register("", &counterPage{}); err == nil {
		t.Fatal("expected empty route error")
	}
	if err := r.Register("/x", nil); err == nil {
		t.Fatal("expected nil page error")
	}
}

func TestAdapterDelegatesToPage(t *testing.T) {
	p := &counterPage{initial: 7}
	a := Adapter{Page: p}
	state, err := p.Init(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	html, err := a.Render(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	if html == "" {
		t.Fatal("empty html")
	}
	state2, err := p.Handle(context.Background(), state, publiclive.Event{Kind: "event", Name: "inc"})
	if err != nil {
		t.Fatal(err)
	}
	if state2.(*counterState).Count != 8 {
		t.Fatalf("want 8 got %d", state2.(*counterState).Count)
	}
}

func TestRoutesList(t *testing.T) {
	r := NewRegistry()
	_ = r.Register("/a", &counterPage{})
	_ = r.Register("/b", &counterPage{})
	got := r.Routes()
	if len(got) != 2 {
		t.Fatalf("want 2 got %d", len(got))
	}
}

