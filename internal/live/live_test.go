package live

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeRenderer struct {
	fn func(any) string
}

func (f *fakeRenderer) Render(_ context.Context, state any) (string, error) {
	return f.fn(state), nil
}

type fakeHandler struct {
	fn func(any, Event) (any, error)
}

func (f *fakeHandler) Handle(_ context.Context, state any, evt Event) (any, error) {
	return f.fn(state, evt)
}

func TestDispatchInitialReplace(t *testing.T) {
	hub := NewHub(Options{})
	defer hub.Stop(context.Background())

	r := &fakeRenderer{fn: func(s any) string {
		return "<h1>" + s.(string) + "</h1>"
	}}
	h := &fakeHandler{fn: func(s any, _ Event) (any, error) {
		return s.(string) + "!", nil
	}}
	sess, err := hub.Spawn(context.Background(), "id1", "/", "hello", r, h)
	if err != nil {
		t.Fatal(err)
	}
	sess.Dispatch(Event{Kind: "event", Name: "noop"})

	select {
	case msg := <-sess.OutboundCh():
		if msg.Kind != "replace" {
			t.Fatalf("first dispatch want replace, got %s", msg.Kind)
		}
		if !strings.Contains(msg.HTML, "hello!") {
			t.Fatalf("html not transformed: %q", msg.HTML)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("no outbound message")
	}
}

func TestDispatchEmitsTextDiff(t *testing.T) {
	hub := NewHub(Options{})
	defer hub.Stop(context.Background())

	count := 0
	r := &fakeRenderer{fn: func(s any) string {
		_ = s
		return "<div><p>count:" + itoaPath(count) + "</p></div>"
	}}
	h := &fakeHandler{fn: func(s any, _ Event) (any, error) {
		count++
		return s, nil
	}}
	sess, _ := hub.Spawn(context.Background(), "id1", "/", nil, r, h)

	sess.Dispatch(Event{Kind: "event"})
	<-sess.OutboundCh() // initial replace

	sess.Dispatch(Event{Kind: "event"})
	select {
	case msg := <-sess.OutboundCh():
		if msg.Kind != "diff" {
			t.Fatalf("want diff, got %s", msg.Kind)
		}
		if len(msg.Diff) == 0 {
			t.Fatal("expected at least 1 op")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("no diff message")
	}
}

func TestHubMaxSessions(t *testing.T) {
	hub := NewHub(Options{MaxSessions: 2})
	defer hub.Stop(context.Background())

	r := &fakeRenderer{fn: func(any) string { return "<p/>" }}
	h := &fakeHandler{fn: func(s any, _ Event) (any, error) { return s, nil }}

	if _, err := hub.Spawn(context.Background(), "a", "/", nil, r, h); err != nil {
		t.Fatal(err)
	}
	if _, err := hub.Spawn(context.Background(), "b", "/", nil, r, h); err != nil {
		t.Fatal(err)
	}
	_, err := hub.Spawn(context.Background(), "c", "/", nil, r, h)
	if !errors.Is(err, ErrTooManySessions) {
		t.Fatalf("want ErrTooManySessions got %v", err)
	}
}

func TestHubGetMissing(t *testing.T) {
	hub := NewHub(Options{})
	defer hub.Stop(context.Background())
	if _, err := hub.Get("nope"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("want ErrSessionNotFound got %v", err)
	}
}

func TestSessionCloseRemovesFromHub(t *testing.T) {
	hub := NewHub(Options{})
	defer hub.Stop(context.Background())

	r := &fakeRenderer{fn: func(any) string { return "<p/>" }}
	h := &fakeHandler{fn: func(s any, _ Event) (any, error) { return s, nil }}
	sess, _ := hub.Spawn(context.Background(), "x", "/", nil, r, h)
	sess.Close()
	// give the cleanup goroutine a tick
	time.Sleep(20 * time.Millisecond)
	if _, err := hub.Get("x"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("session should be removed after close")
	}
}

func TestHandlerErrorBecomesOutboundError(t *testing.T) {
	hub := NewHub(Options{})
	defer hub.Stop(context.Background())

	r := &fakeRenderer{fn: func(any) string { return "<p/>" }}
	h := &fakeHandler{fn: func(any, Event) (any, error) {
		return nil, errors.New("boom")
	}}
	sess, _ := hub.Spawn(context.Background(), "x", "/", nil, r, h)
	sess.Dispatch(Event{Kind: "event"})
	select {
	case msg := <-sess.OutboundCh():
		if msg.Kind != "error" {
			t.Fatalf("want error got %s", msg.Kind)
		}
		if !strings.Contains(msg.Err, "boom") {
			t.Fatalf("err missing reason: %s", msg.Err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("no message")
	}
}

func TestEncodeOutbound(t *testing.T) {
	cases := []Outbound{
		{Kind: "diff", Diff: []Op{{Op: "text", Path: "0", Value: "hi"}}},
		{Kind: "replace", HTML: "<p>x</p>"},
		{Kind: "ping"},
		{Kind: "error", Err: "nope"},
	}
	for _, c := range cases {
		buf, err := EncodeOutbound(c)
		if err != nil {
			t.Errorf("encode %s: %v", c.Kind, err)
			continue
		}
		var probe map[string]any
		if err := json.Unmarshal(buf, &probe); err != nil {
			t.Errorf("invalid json for %s: %v", c.Kind, err)
		}
	}
}

func TestDecodeEvent(t *testing.T) {
	e, err := DecodeEvent([]byte(`{"t":"event","name":"inc","target":"#b"}`))
	if err != nil {
		t.Fatal(err)
	}
	if e.Kind != "event" || e.Name != "inc" || e.Target != "#b" {
		t.Fatalf("decoded %+v", e)
	}
	if _, err := DecodeEvent([]byte(`{}`)); err == nil {
		t.Fatal("expected error for missing kind")
	}
}

func TestDiffNoChange(t *testing.T) {
	if got := Diff("<p>same</p>", "<p>same</p>"); got != nil {
		t.Fatalf("want nil got %v", got)
	}
}

func TestDiffTextChange(t *testing.T) {
	ops := Diff("<p>a</p>", "<p>b</p>")
	if len(ops) != 1 {
		t.Fatalf("want 1 op got %d: %v", len(ops), ops)
	}
	if ops[0].Op != "text" {
		t.Fatalf("want text op got %s", ops[0].Op)
	}
}

func TestDiffAttrChange(t *testing.T) {
	ops := Diff(`<p class="a">x</p>`, `<p class="b">x</p>`)
	if len(ops) != 1 || ops[0].Op != "attr" {
		t.Fatalf("want single attr op got %v", ops)
	}
	if ops[0].Attrs["class"] != "b" {
		t.Fatalf("attr missing: %v", ops[0].Attrs)
	}
}

func TestDiffStructuralFallback(t *testing.T) {
	ops := Diff(`<p>x</p>`, `<div>x</div>`)
	if len(ops) != 1 || ops[0].Op != "replace" {
		t.Fatalf("want replace got %v", ops)
	}
}

func TestDiffChildLengthDiffersFallsBackToReplace(t *testing.T) {
	ops := Diff(`<ul><li>a</li></ul>`, `<ul><li>a</li><li>b</li></ul>`)
	if len(ops) == 0 {
		t.Fatal("expected at least one op")
	}
	if ops[0].Op != "replace" {
		t.Fatalf("first op should be replace got %s", ops[0].Op)
	}
}

func TestSessionTouch(t *testing.T) {
	hub := NewHub(Options{IdleTimeout: 50 * time.Millisecond})
	defer hub.Stop(context.Background())

	r := &fakeRenderer{fn: func(any) string { return "<p/>" }}
	h := &fakeHandler{fn: func(s any, _ Event) (any, error) { return s, nil }}
	sess, _ := hub.Spawn(context.Background(), "id", "/", nil, r, h)

	var dispatches int32
	go func() {
		for i := 0; i < 5; i++ {
			sess.Dispatch(Event{Kind: "event"})
			atomic.AddInt32(&dispatches, 1)
			time.Sleep(20 * time.Millisecond)
		}
	}()

	go hub.Run(sess.Context())
	time.Sleep(150 * time.Millisecond)
	// session should still be alive because dispatch keeps touching it
	if _, err := hub.Get("id"); err != nil {
		t.Fatalf("session was reaped while active: %v", err)
	}
}
