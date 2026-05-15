package observe

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/dev2k6/Nguyen.go/pkg/ngctx"
)

func newCapture() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	l := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return l, &buf
}

func TestLoggerInheritsCtxAttrs(t *testing.T) {
	l, buf := newCapture()
	SetDefault(l)

	ctx := context.Background()
	ctx = ngctx.WithTraceID(ctx, "trace-1")
	ctx = ngctx.WithRequestID(ctx, "req-1")
	ctx = ngctx.WithLocale(ctx, "vi-VN")
	ctx = ngctx.WithUser(ctx, &ngctx.UserInfo{ID: "u-1"})

	Logger(ctx).Info("hello")
	out := buf.String()
	for _, want := range []string{"trace_id=trace-1", "request_id=req-1", "locale=vi-VN", "user_id=u-1", "msg=hello"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
}

func TestWithLoggerOverride(t *testing.T) {
	l, buf := newCapture()
	custom := l.With("custom", "yes")
	ctx := WithLogger(context.Background(), custom)
	Logger(ctx).Info("x")
	if !strings.Contains(buf.String(), "custom=yes") {
		t.Fatalf("expected custom attr in %s", buf.String())
	}
}

func TestSpanLogsDuration(t *testing.T) {
	l, buf := newCapture()
	SetDefault(l)
	ctx := ngctx.WithTraceID(context.Background(), "tid")
	sp := StartSpan(ctx, "load.posts")
	sp.SetAttr("count", 3)
	sp.End(nil)
	out := buf.String()
	if !strings.Contains(out, "span=load.posts") {
		t.Errorf("missing span name: %s", out)
	}
	if !strings.Contains(out, "count=3") {
		t.Errorf("missing attr: %s", out)
	}
	if !strings.Contains(out, "duration=") {
		t.Errorf("missing duration: %s", out)
	}
}

func TestSpanLogsError(t *testing.T) {
	l, buf := newCapture()
	SetDefault(l)
	sp := StartSpan(context.Background(), "fail")
	sp.End(errors.New("nope"))
	out := buf.String()
	if !strings.Contains(out, "level=ERROR") {
		t.Errorf("expected error level: %s", out)
	}
	if !strings.Contains(out, `error=nope`) {
		t.Errorf("expected error attr: %s", out)
	}
}

func TestNewJSONHandler(t *testing.T) {
	var buf bytes.Buffer
	l := NewJSON(&buf, slog.LevelInfo)
	l.Info("hello", "k", "v")
	if !strings.Contains(buf.String(), `"msg":"hello"`) {
		t.Fatalf("not json: %s", buf.String())
	}
}

func TestSetDefaultNilResets(t *testing.T) {
	SetDefault(nil)
	if Default() == nil {
		t.Fatal("default logger should not be nil after reset")
	}
}

func TestCallerReturnsFileLine(t *testing.T) {
	got := Caller(0)
	if !strings.Contains(got, "observe_test.go:") {
		t.Fatalf("expected file marker, got %s", got)
	}
}
