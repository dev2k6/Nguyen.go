package router

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// SegmentType classifies a route segment
type SegmentType int

const (
	Static   SegmentType = iota // static name like "about"
	Dynamic                     // [param] dynamic
	CatchAll                    // [...] spread
	Optional                    // [[...param]] optional catch-all
)

// Segment represents one piece of a route path
type Segment struct {
	Name      string
	Type      SegmentType
	ParamName string // extracted from brackets, e.g. "slug" from "[slug]"
}

// Route describes one resolved page route
type Route struct {
	FilePath  string         // absolute path to .gox file
	Pattern   string         // Fiber-compatible pattern: /blog/:slug
	Segments  []Segment      // parsed segments
	Is404     bool           // true for 404.gox
	IsLayout  bool           // true for layout.gox files
	Priority  int            // lower = evaluated first (static > dynamic > catch-all)
	GuardName string         // route guard name from frontmatter (guard = "auth")
	Regex     *regexp.Regexp // pre-compiled regex for O(1) request matching (Astro-style)
	ParamNames []string      // ordered param names matching regex capture groups
}

// LayoutInfo describes a layout file discovered in a directory.
type LayoutInfo struct {
	FilePath string // absolute path to layout file
	Depth    int    // directory depth (0 = root, 1 = blog/, etc.)
	DirPath  string // directory containing this layout
}

// FindLayouts discovers all layout.gox files and their hierarchical order.
// Returns sorted by depth (shallowest first).
func FindLayouts(rootDir string) ([]LayoutInfo, error) {
	var layouts []LayoutInfo

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		base := strings.TrimSuffix(info.Name(), ".gox")
		if base != "layout" {
			return nil
		}

		dir := filepath.Dir(path)
		rel, _ := filepath.Rel(rootDir, dir)
		rel = filepath.ToSlash(rel)

		depth := 0
		if rel != "." && rel != "" {
			depth = len(strings.Split(rel, "/"))
		}

		layouts = append(layouts, LayoutInfo{
			FilePath: path,
			Depth:    depth,
			DirPath:  dir,
		})
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("find layouts: %w", err)
	}

	// Sort by depth ascending (root layout first, then deeper)
	sort.Slice(layouts, func(i, j int) bool {
		return layouts[i].Depth < layouts[j].Depth
	})

	return layouts, nil
}

// GetLayoutsForRoute returns layout files that apply to a given page.
// Layouts are matched by directory nesting: /pages/layout.gox applies
// to all pages, /pages/blog/layout.gox applies to /pages/blog/*.
func GetLayoutsForRoute(pagePath string, layouts []LayoutInfo) []LayoutInfo {
	var applicable []LayoutInfo
	pageDir := filepath.ToSlash(filepath.Dir(pagePath))

	for _, l := range layouts {
		lDir := filepath.ToSlash(l.DirPath)
		// Check if layout dir is a prefix of page dir
		if strings.HasPrefix(pageDir, lDir) || lDir == "." {
			applicable = append(applicable, l)
		}
	}
	return applicable
}

// Discover walks the pages directory and resolves all .gox files into routes.
// Implements file-system routing:
//
//	pages/index.gox       → /
//	pages/about.gox       → /about
//	pages/blog/[slug].gox → /blog/:slug
//	pages/404.gox         → catch-all fallback
func Discover(rootDir string) ([]Route, error) {
	var routes []Route

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".gox") {
			return nil
		}

		route, err := resolveRoute(rootDir, path, info)
		if err != nil {
			return fmt.Errorf("in %s: %w", path, err)
		}
		if route.Pattern == "" {
			return nil // skip internal files (_app, layout)
		}
		routes = append(routes, route)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("router: walk failed: %w", err)
	}

	// Sort by priority so static routes evaluate first
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Priority < routes[j].Priority
	})

	return routes, nil
}

// resolveRoute converts one file path into a Route
func resolveRoute(rootDir, fullPath string, info os.FileInfo) (Route, error) {
	// Relative path from root, minus .gox extension
	rel, err := filepath.Rel(rootDir, fullPath)
	if err != nil {
		return Route{}, err
	}
	rel = filepath.ToSlash(rel)
	rel = strings.TrimSuffix(rel, ".gox")

	// Special files
	base := filepath.Base(fullPath)
	base = strings.TrimSuffix(base, ".gox")

	// 404 page
	if base == "404" {
		return Route{
			FilePath: fullPath,
			Pattern:  "/*",
			Segments: []Segment{{Name: "*", Type: CatchAll}},
			Is404:    true,
			Priority: 1000, // lowest priority
		}, nil
	}

	// _app and layout are internal files, not routes
	if strings.HasPrefix(base, "_") || base == "layout" {
		return Route{}, nil
	}

	// Parse segments
	parts := strings.Split(rel, "/")
	// Remove "pages" prefix if present
	if len(parts) > 0 && parts[0] == "pages" {
		parts = parts[1:]
	}

	segments := parseSegments(parts)
	pattern := buildPattern(parts)
	priority := computePriority(segments)
	rx, params := buildRegex(segments)

	return Route{
		FilePath:   fullPath,
		Pattern:    pattern,
		Segments:   segments,
		Priority:   priority,
		Regex:      rx,
		ParamNames: params,
	}, nil
}

// buildRegex compiles segments into a single regex plus the ordered list
// of param capture names. Modeled on Astro's getPattern (pattern.ts:4-46):
//   - static segments are regex-escaped literal matches
//   - dynamic [param] → ([^/]+?)
//   - catch-all [...param] → (.*?)
//   - optional [[...param]] → (?:/(.*?))?
//
// The trailing $ anchors to end-of-path so /about does not match /about/x.
func buildRegex(segments []Segment) (*regexp.Regexp, []string) {
	if len(segments) == 0 {
		return regexp.MustCompile(`^/?$`), nil
	}

	var sb strings.Builder
	sb.WriteString("^")
	var params []string

	for _, seg := range segments {
		switch seg.Type {
		case Static:
			// index segment maps to root — already handled in buildPattern, but
			// when it appears here it represents a literal path component.
			if seg.Name == "index" {
				continue
			}
			sb.WriteString("/")
			sb.WriteString(regexp.QuoteMeta(seg.Name))
		case Dynamic:
			sb.WriteString("/([^/]+?)")
			params = append(params, seg.ParamName)
		case CatchAll:
			sb.WriteString("/(.*?)")
			params = append(params, seg.ParamName)
		case Optional:
			sb.WriteString("(?:/(.*?))?")
			params = append(params, seg.ParamName)
		}
	}
	sb.WriteString("/?$")

	compiled, err := regexp.Compile(sb.String())
	if err != nil {
		return nil, nil
	}
	return compiled, params
}

// Match returns (params, true) if reqPath matches the route pattern, else nil.
// Uses the pre-compiled Regex field for O(1) per-route matching.
func (r *Route) Match(reqPath string) (map[string]string, bool) {
	if r.Regex == nil {
		return nil, false
	}
	matches := r.Regex.FindStringSubmatch(reqPath)
	if matches == nil {
		return nil, false
	}
	params := make(map[string]string, len(r.ParamNames))
	for i, name := range r.ParamNames {
		if i+1 < len(matches) {
			params[name] = matches[i+1]
		}
	}
	return params, true
}

// MatchRoute walks sorted routes and returns the first match for a path.
// Routes must already be sorted by Priority (Discover does this).
func MatchRoute(routes []Route, reqPath string) (*Route, map[string]string, bool) {
	for i := range routes {
		if params, ok := routes[i].Match(reqPath); ok {
			return &routes[i], params, true
		}
	}
	return nil, nil, false
}

// parseSegments converts path parts into typed Segments
func parseSegments(parts []string) []Segment {
	var segs []Segment
	for _, p := range parts {
		seg := parseSegment(p)
		segs = append(segs, seg)
	}
	return segs
}

// parseSegment parses a single path part
func parseSegment(part string) Segment {
	// [...param]
	if strings.HasPrefix(part, "[...") && strings.HasSuffix(part, "]") {
		inner := part[4 : len(part)-1]
		return Segment{Name: part, ParamName: inner, Type: CatchAll}
	}
	// [[...param]]
	if strings.HasPrefix(part, "[[...") && strings.HasSuffix(part, "]]") {
		inner := part[5 : len(part)-2]
		return Segment{Name: part, ParamName: inner, Type: Optional}
	}
	// [param]
	if strings.HasPrefix(part, "[") && strings.HasSuffix(part, "]") {
		inner := part[1 : len(part)-1]
		return Segment{Name: part, ParamName: inner, Type: Dynamic}
	}
	return Segment{Name: part, Type: Static}
}

// buildPattern converts path parts into a Fiber-compatible pattern
func buildPattern(parts []string) string {
	if len(parts) == 0 || parts[0] == "" {
		return "/"
	}

	// index file maps to root
	if len(parts) == 1 && parts[0] == "index" {
		return "/"
	}

	var sb strings.Builder
	for _, p := range parts {
		sb.WriteByte('/')
		seg := parseSegment(p)
		switch seg.Type {
		case Dynamic:
			sb.WriteByte(':')
			sb.WriteString(seg.ParamName)
		case CatchAll:
			sb.WriteByte('*')
		case Optional:
			sb.WriteByte('*')
		default:
			sb.WriteString(seg.Name)
		}
	}
	return sb.String()
}

// computePriority assigns lower numbers to more specific routes
func computePriority(segments []Segment) int {
	score := 0
	for _, seg := range segments {
		switch seg.Type {
		case Static:
			score += 0
		case Dynamic:
			score += 10
		case CatchAll:
			score += 100
		case Optional:
			score += 50
		}
	}
	return score
}

// SortRoutes ensures the most specific routes are evaluated first
func SortRoutes(routes []Route) {
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Priority < routes[j].Priority
	})
}

// dynamicParamRx validates dynamic segment names
var dynamicParamRx = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// ValidateSegments checks that dynamic segment names are valid Go identifiers
func ValidateSegments(segments []Segment) error {
	for _, seg := range segments {
		if seg.Type == Dynamic || seg.Type == CatchAll || seg.Type == Optional {
			if !dynamicParamRx.MatchString(seg.ParamName) {
				return fmt.Errorf("invalid dynamic parameter name %q: must be a valid identifier", seg.ParamName)
			}
		}
	}
	return nil
}
