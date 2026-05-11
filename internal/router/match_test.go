package router

import (
	"testing"
)

// Tests for pre-compiled route regex matching (Astro-style)
func TestRouteMatch_Static(t *testing.T) {
	segs := []Segment{
		{Name: "about", Type: Static},
	}
	rx, params := buildRegex(segs)
	r := &Route{Regex: rx, ParamNames: params}

	if got, ok := r.Match("/about"); !ok || len(got) != 0 {
		t.Errorf("expected match for /about, got %v ok=%v", got, ok)
	}
	if _, ok := r.Match("/about/x"); ok {
		t.Errorf("unexpected match for /about/x")
	}
}

func TestRouteMatch_Dynamic(t *testing.T) {
	segs := []Segment{
		{Name: "blog", Type: Static},
		{Name: "[slug]", Type: Dynamic, ParamName: "slug"},
	}
	rx, params := buildRegex(segs)
	r := &Route{Regex: rx, ParamNames: params}

	got, ok := r.Match("/blog/hello-world")
	if !ok {
		t.Fatalf("expected match for /blog/hello-world")
	}
	if got["slug"] != "hello-world" {
		t.Errorf("expected slug=hello-world, got %q", got["slug"])
	}
	if _, ok := r.Match("/blog"); ok {
		t.Errorf("unexpected match for /blog without slug")
	}
}

func TestRouteMatch_CatchAll(t *testing.T) {
	segs := []Segment{
		{Name: "docs", Type: Static},
		{Name: "[...path]", Type: CatchAll, ParamName: "path"},
	}
	rx, params := buildRegex(segs)
	r := &Route{Regex: rx, ParamNames: params}

	got, ok := r.Match("/docs/guide/intro")
	if !ok {
		t.Fatalf("expected match for /docs/guide/intro")
	}
	if got["path"] != "guide/intro" {
		t.Errorf("expected path=guide/intro, got %q", got["path"])
	}
}

func TestRouteMatch_OrderedByPriority(t *testing.T) {
	routes := []Route{
		{
			Pattern:    "/blog/:slug",
			Segments:   []Segment{{Name: "blog", Type: Static}, {Name: "[slug]", Type: Dynamic, ParamName: "slug"}},
			Priority:   10,
		},
		{
			Pattern:  "/blog/featured",
			Segments: []Segment{{Name: "blog", Type: Static}, {Name: "featured", Type: Static}},
			Priority: 0,
		},
	}
	// Pre-compile regexes
	for i := range routes {
		routes[i].Regex, routes[i].ParamNames = buildRegex(routes[i].Segments)
	}

	// Sort so static (priority 0) comes first
	SortRoutes(routes)

	matched, params, ok := MatchRoute(routes, "/blog/featured")
	if !ok {
		t.Fatalf("expected match for /blog/featured")
	}
	if matched.Pattern != "/blog/featured" {
		t.Errorf("expected static route to win, got %q (params=%v)", matched.Pattern, params)
	}
}
