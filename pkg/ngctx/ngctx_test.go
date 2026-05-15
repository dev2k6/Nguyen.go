package ngctx

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestTraceID(t *testing.T) {
	ctx := WithTraceID(context.Background(), "abc")
	if got := TraceID(ctx); got != "abc" {
		t.Fatalf("want abc got %s", got)
	}
	if got := TraceID(context.Background()); got != "" {
		t.Fatalf("want empty got %s", got)
	}
	ctx = WithTraceID(context.Background(), "")
	if got := TraceID(ctx); got != "" {
		t.Fatalf("empty trace id should not be stored, got %s", got)
	}
}

func TestRequestID(t *testing.T) {
	ctx := WithRequestID(context.Background(), "r-1")
	if got := RequestID(ctx); got != "r-1" {
		t.Fatalf("want r-1 got %s", got)
	}
}

func TestLocale(t *testing.T) {
	ctx := WithLocale(context.Background(), "vi-VN")
	if got := Locale(ctx); got != "vi-VN" {
		t.Fatalf("want vi-VN got %s", got)
	}
}

func TestUserAndRoles(t *testing.T) {
	u := &UserInfo{ID: "1", Email: "a@b", Roles: []string{"admin", "ops"}}
	ctx := WithUser(context.Background(), u)
	got := User(ctx)
	if got == nil || got.ID != "1" {
		t.Fatalf("want user 1 got %+v", got)
	}
	if !got.HasRole("admin") {
		t.Fatal("admin role missing")
	}
	if got.HasRole("nope") {
		t.Fatal("nope should not be present")
	}
	var nilUser *UserInfo
	if nilUser.HasRole("any") {
		t.Fatal("nil user must not have role")
	}
}

func TestBudget(t *testing.T) {
	t0 := time.Now().Add(time.Second)
	ctx := WithBudget(context.Background(), t0)
	if got := Budget(ctx); !got.Equal(t0) {
		t.Fatalf("want %v got %v", t0, got)
	}
}

func TestNewIDIsHex(t *testing.T) {
	id := NewID()
	if len(id) != 32 {
		t.Fatalf("want 32-char hex got %d (%q)", len(id), id)
	}
	for _, c := range id {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			t.Fatalf("non-hex char %q in %q", c, id)
		}
	}
}

func TestFromHTTPGeneratesIDs(t *testing.T) {
	h := http.Header{}
	ctx := FromHTTP(context.Background(), h)
	if TraceID(ctx) == "" || RequestID(ctx) == "" {
		t.Fatal("trace/request id should be generated")
	}
}

func TestFromHTTPHonoursHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("X-Trace-Id", "incoming-trace")
	h.Set("X-Request-Id", "incoming-req")
	h.Set("Accept-Language", "vi-VN,vi;q=0.9,en;q=0.8")
	ctx := FromHTTP(context.Background(), h)
	if TraceID(ctx) != "incoming-trace" {
		t.Fatalf("trace got %s", TraceID(ctx))
	}
	if RequestID(ctx) != "incoming-req" {
		t.Fatalf("request got %s", RequestID(ctx))
	}
	if Locale(ctx) != "vi-VN" {
		t.Fatalf("locale got %s", Locale(ctx))
	}
}

func TestPrimaryLanguageEdgeCases(t *testing.T) {
	cases := map[string]string{
		"vi-VN":             "vi-VN",
		"vi-VN,en;q=0.9":    "vi-VN",
		"en;q=0.5":          "en",
		"":                  "",
		"  en":              "",
	}
	for in, want := range cases {
		if got := primaryLanguage(in); got != want {
			t.Errorf("primaryLanguage(%q) want %q got %q", in, want, got)
		}
	}
}
