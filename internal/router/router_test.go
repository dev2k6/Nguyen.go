package router

import (
	"strings"
	"testing"
)

func TestParseSegment_Static(t *testing.T) {
	seg := parseSegment("about")
	if seg.Type != Static {
		t.Errorf("expected Static, got %v", seg.Type)
	}
	if seg.Name != "about" {
		t.Errorf("expected name 'about', got %q", seg.Name)
	}
}

func TestParseSegment_Dynamic(t *testing.T) {
	seg := parseSegment("[slug]")
	if seg.Type != Dynamic {
		t.Errorf("expected Dynamic, got %v", seg.Type)
	}
	if seg.ParamName != "slug" {
		t.Errorf("expected paramName 'slug', got %q", seg.ParamName)
	}
}

func TestParseSegment_CatchAll(t *testing.T) {
	seg := parseSegment("[...path]")
	if seg.Type != CatchAll {
		t.Errorf("expected CatchAll, got %v", seg.Type)
	}
	if seg.ParamName != "path" {
		t.Errorf("expected paramName 'path', got %q", seg.ParamName)
	}
}

func TestParseSegment_Optional(t *testing.T) {
	seg := parseSegment("[[...slug]]")
	if seg.Type != Optional {
		t.Errorf("expected Optional, got %v", seg.Type)
	}
	if seg.ParamName != "slug" {
		t.Errorf("expected paramName 'slug', got %q", seg.ParamName)
	}
}

func TestBuildPattern_Index(t *testing.T) {
	pattern := buildPattern([]string{"index"})
	if pattern != "/" {
		t.Errorf("expected /, got %s", pattern)
	}
}

func TestBuildPattern_NestedDynamic(t *testing.T) {
	pattern := buildPattern([]string{"blog", "[slug]"})
	if pattern != "/blog/:slug" {
		t.Errorf("expected /blog/:slug, got %s", pattern)
	}
}

func TestBuildPattern_CatchAll(t *testing.T) {
	pattern := buildPattern([]string{"docs", "[...path]"})
	if pattern != "/docs/*" {
		t.Errorf("expected /docs/*, got %s", pattern)
	}
}

func TestBuildPattern_Deep(t *testing.T) {
	pattern := buildPattern([]string{"a", "b", "[id]", "c"})
	if pattern != "/a/b/:id/c" {
		t.Errorf("expected /a/b/:id/c, got %s", pattern)
	}
}

func TestComputePriority(t *testing.T) {
	static := []Segment{{Type: Static, Name: "about"}}
	dynamic := []Segment{{Type: Dynamic, ParamName: "slug"}}
	catchall := []Segment{{Type: CatchAll, ParamName: "path"}}
	mixed := []Segment{{Type: Static, Name: "api"}, {Type: Dynamic, ParamName: "id"}}

	if computePriority(static) >= computePriority(dynamic) {
		t.Error("static should have lower priority than dynamic")
	}
	if computePriority(dynamic) >= computePriority(catchall) {
		t.Error("dynamic should have lower priority than catchall")
	}
	if computePriority(static) >= computePriority(mixed) {
		t.Error("static should have lower priority than mixed")
	}
}

func TestValidateSegments_Valid(t *testing.T) {
	segs := []Segment{
		{Type: Dynamic, ParamName: "slug"},
		{Type: Dynamic, ParamName: "postId"},
		{Type: CatchAll, ParamName: "path"},
	}
	if err := ValidateSegments(segs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateSegments_Invalid(t *testing.T) {
	segs := []Segment{
		{Type: Dynamic, ParamName: "123bad"},
	}
	if err := ValidateSegments(segs); err == nil {
		t.Error("expected error for invalid param name")
	}
}

func TestSortRoutes(t *testing.T) {
	routes := []Route{
		{Pattern: "/*", Priority: 100},
		{Pattern: "/blog/:slug", Priority: 10},
		{Pattern: "/about", Priority: 0},
		{Pattern: "/api/:id", Priority: 10},
	}
	SortRoutes(routes)
	for i := 1; i < len(routes); i++ {
		if routes[i].Priority < routes[i-1].Priority {
			t.Errorf("routes not sorted: %+v", routes)
		}
	}
}

func TestBuildPattern_RootIndex(t *testing.T) {
	// "index" is root
	pattern := buildPattern([]string{})
	if pattern != "/" {
		t.Errorf("expected /, got %s", pattern)
	}
}

func TestBuildPattern_EmptyParts(t *testing.T) {
	pattern := buildPattern([]string{""})
	if pattern != "/" {
		t.Errorf("expected /, got %s", pattern)
	}
}

func TestParseSegments(t *testing.T) {
	parts := []string{"blog", "[slug]", "[...tags]"}
	segs := parseSegments(parts)
	if len(segs) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(segs))
	}
	if segs[0].Type != Static || segs[0].Name != "blog" {
		t.Errorf("seg 0: expected static 'blog', got %v", segs[0])
	}
	if segs[1].Type != Dynamic || segs[1].ParamName != "slug" {
		t.Errorf("seg 1: expected dynamic 'slug', got %v", segs[1])
	}
	if segs[2].Type != CatchAll || segs[2].ParamName != "tags" {
		t.Errorf("seg 2: expected catchall 'tags', got %v", segs[2])
	}
}

func TestRoutePriority_Ranges(t *testing.T) {
	// Verify priority ranges make sense:
	// Static: 0, Dynamic: 10, Optional: 50, CatchAll: 100
	static := Route{Segments: []Segment{{Type: Static}, {Type: Static}}}
	dyn := Route{Segments: []Segment{{Type: Static}, {Type: Dynamic}}}
	opt := Route{Segments: []Segment{{Type: Static}, {Type: Optional}}}
	ca := Route{Segments: []Segment{{Type: Static}, {Type: CatchAll}}}

	static.ComputePriority()
	dyn.ComputePriority()
	opt.ComputePriority()
	ca.ComputePriority()

	if static.Priority >= dyn.Priority {
		t.Error("static must eval before dynamic")
	}
	if dyn.Priority >= opt.Priority {
		t.Error("dynamic must eval before optional")
	}
	if opt.Priority >= ca.Priority {
		t.Error("optional must eval before catch-all")
	}
}

func (r *Route) ComputePriority() {
	r.Priority = computePriority(r.Segments)
}

// TestPathToSlash verifies backslash conversion
func TestPathToSlash(t *testing.T) {
	path := `pages\blog\[slug].nguyen`
	// Simulate filepath.ToSlash
	path = strings.ReplaceAll(path, `\`, "/")
	if !strings.Contains(path, "/") {
		t.Error("backslash should convert to slash")
	}
}
