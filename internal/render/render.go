package render

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dev2k6/Nguyen.go/internal/geo"
	"github.com/dev2k6/Nguyen.go/internal/parser"
	"github.com/dev2k6/Nguyen.go/internal/router"
)

// Result holds the output of server-side rendering
type Result struct {
	HTML        string            // Complete HTML output
	Meta        map[string]string // Extracted metadata
	GEO         *geo.PageGEO      // Extracted GEO metadata
	State       map[string]string // Computed initial state values
	Revalidate  int               // ISR revalidate time in seconds (0 = no cache)
	Version     string            // Framework version string
	ScopedCSS   *parser.ScopedCSS // Scoped CSS data if present
	ImageData   map[string]string // src → srcset string for responsive images
	MetaTags    string            // Pre-built <meta> block for injection into <head>
	CriticalCSS string            // Inlined critical CSS
	DeferredCSS string            // Deferred CSS URL path
}

var (
	varRx         = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)
	revalidateRx  = regexp.MustCompile(`export\s+const\s+revalidate\s*=\s*(\d+)`)
	layoutRx      = regexp.MustCompile(`layout\s*=\s*"([^"]+)"`)
	slotRx        = regexp.MustCompile(`<nguyen-slot\s*/?\s*>`)
	slotNamedRx   = regexp.MustCompile(`<nguyen-slot\s+name="([^"]+)"\s*/?\s*>`)
	eventAttrRx   = regexp.MustCompile(`\s@(\w+)="([^"]+)"`)
	funcRx        = regexp.MustCompile(`func\s+(\w+)\s*\(\s*\)\s*\{([^}]*)\}`)
	setStateRx    = regexp.MustCompile(`core\.SetGlobalState\("(\w+)",\s*(.+?)\)`)
	getStateRx    = regexp.MustCompile(`core\.GetGlobalState\("(\w+)"\)\.\(int\)`)
	nguyenLinkRx  = regexp.MustCompile(`<nguyen-link\b([^>]*)>`)
	nguyenLinkEnd = regexp.MustCompile(`</nguyen-link>`)
)

// RenderSSR performs server-side rendering of a .gox file.
// It processes the template, replaces {variable} interpolations with
// computed state values, injects metadata into <head>, and returns
// the complete HTML.
func RenderSSR(f *parser.File, version string, layouts ...router.LayoutInfo) *Result {
	result := &Result{
		Meta:    make(map[string]string),
		State:   make(map[string]string),
		Version: version,
	}

	// Extract revalidate time
	if m := revalidateRx.FindStringSubmatch(f.GoCode); len(m) > 1 {
		fmt.Sscanf(m[1], "%d", &result.Revalidate)
	}

	// Extract metadata
	extractMetadata(f.GoCode, result.Meta)

	// Extract GEO metadata
	result.GEO = extractGEO(f.GoCode, result.Meta)

	// Extract state initial values
	extractState(f.GoCode, result.State)

	// Build HTML from template
	html := f.HTMLTemplate

	// Process fragments (remove wrapper tags)
	html = ProcessFragments(html)

	// Replace {variable} with reactive hydration spans.
	// Each known state variable gets a <span data-nguyen-text="key">value</span>
	// so the client-side JS can update it when state changes.
	// Unknown variables are replaced with empty string.
	html = varRx.ReplaceAllStringFunc(html, func(match string) string {
		name := match[1 : len(match)-1] // strip {}
		if val, ok := result.State[name]; ok {
			return fmt.Sprintf(`<span data-nguyen-text=%q>%s</span>`, name, htmlEscape(val))
		}
		return "" // unknown vars → empty
	})

	// Transform <nguyen-link> → <a> for SSR output
	html = nguyenLinkRx.ReplaceAllString(html, `<a$1>`)
	html = nguyenLinkEnd.ReplaceAllString(html, `</a>`)

	// Process conditional directives (<nguyen-show>, <nguyen-hide>)
	html = ProcessConditionals(html, result.State)

	// Compose with layout if declared in frontmatter or nested layouts discovered
	html = composeLayout(f, html, layouts)

	// SSR hydration: add event handler markers and inject reactive JS bridge.
	html = injectHydrationMarkers(html, result.State)

	// Build meta tag block so callers can inject them into <head> themselves.
	result.HTML = html
	result.ScopedCSS = f.ScopedCSS
	result.ImageData = make(map[string]string)
	result.MetaTags = buildMetaTagHTML(result)
	return result
}

// extractMetadata parses `export const metadata = { ... }` from Go code
func extractMetadata(goCode string, meta map[string]string) {
	idx := strings.Index(goCode, "export const metadata")
	if idx == -1 {
		return
	}

	start := strings.Index(goCode[idx:], "{")
	if start == -1 {
		return
	}
	start += idx
	end := strings.Index(goCode[start:], "}")
	if end == -1 {
		return
	}
	end += start + 1

	block := goCode[start:end]

	kvRx := regexp.MustCompile(`(\w+):\s*"([^"]*)"`)
	matches := kvRx.FindAllStringSubmatch(block, -1)
	for _, m := range matches {
		meta[m[1]] = m[2]
	}
}

// extractGEO parses `export const geo = { ... }` from Go code
func extractGEO(goCode string, meta map[string]string) *geo.PageGEO {
	idx := strings.Index(goCode, "export const geo")
	if idx == -1 {
		return nil
	}

	start := strings.Index(goCode[idx:], "{")
	if start == -1 {
		return nil
	}
	start += idx
	end := findMatchingBrace(goCode, start)
	if end == -1 {
		return nil
	}

	block := goCode[start : end+1]
	pg := &geo.PageGEO{}

	// Extract simple string fields: pageType, datePublished, dateModified, author
	pg.PageType = extractStringField(block, "pageType")
	pg.DatePublished = extractStringField(block, "datePublished")
	pg.DateModified = extractStringField(block, "dateModified")
	pg.Author = extractStringField(block, "author")

	// Fill from metadata if not in geo
	if pg.Title == "" {
		pg.Title = meta["title"]
	}
	if pg.Description == "" {
		pg.Description = meta["description"]
	}

	// Extract speakable array
	pg.SpeakableSelectors = extractStringArrayField(block, "speakable")

	// Extract sameAs array
	pg.SameAs = extractStringArrayField(block, "sameAs")

	// Extract images array
	pg.Images = extractStringArrayField(block, "images")

	// Extract tags array
	pg.Tags = extractStringArrayField(block, "tags")

	// Extract breadcrumbs
	breadcrumbRx := regexp.MustCompile(`\{name:\s*"([^"]*)",\s*url:\s*"([^"]*)"\}`)
	bcIdx := strings.Index(block, "breadcrumbs")
	if bcIdx != -1 {
		bcStart := strings.Index(block[bcIdx:], "[")
		if bcStart != -1 {
			bcStart += bcIdx
			bcEnd := findMatchingBracket(block, bcStart)
			if bcEnd != -1 {
				matches := breadcrumbRx.FindAllStringSubmatch(block[bcStart:bcEnd+1], -1)
				for _, m := range matches {
					pg.Breadcrumbs = append(pg.Breadcrumbs, geo.BreadcrumbItem{Name: m[1], URL: m[2]})
				}
			}
		}
	}

	// Extract FAQs
	faqRx := regexp.MustCompile(`\{question:\s*"([^"]*)",\s*answer:\s*"([^"]*)"\}`)
	faqIdx := strings.Index(block, "faqs")
	if faqIdx != -1 {
		faqStart := strings.Index(block[faqIdx:], "[")
		if faqStart != -1 {
			faqStart += faqIdx
			faqEnd := findMatchingBracket(block, faqStart)
			if faqEnd != -1 {
				matches := faqRx.FindAllStringSubmatch(block[faqStart:faqEnd+1], -1)
				for _, m := range matches {
					pg.Faqs = append(pg.Faqs, geo.FAQItem{Question: m[1], Answer: m[2]})
				}
			}
		}
	}

	// Extract HowTo steps
	stepRx := regexp.MustCompile(`\{name:\s*"([^"]*)",\s*text:\s*"([^"]*)"\}`)
	stepIdx := strings.Index(block, "howToSteps")
	if stepIdx != -1 {
		stepStart := strings.Index(block[stepIdx:], "[")
		if stepStart != -1 {
			stepStart += stepIdx
			stepEnd := findMatchingBracket(block, stepStart)
			if stepEnd != -1 {
				matches := stepRx.FindAllStringSubmatch(block[stepStart:stepEnd+1], -1)
				for _, m := range matches {
					pg.HowToSteps = append(pg.HowToSteps, geo.HowToStep{Name: m[1], Text: m[2]})
				}
			}
		}
	}

	return pg
}

func extractStringField(block, fieldName string) string {
	rx := regexp.MustCompile(fieldName + `:\s*"([^"]*)"`)
	if m := rx.FindStringSubmatch(block); len(m) > 1 {
		return m[1]
	}
	return ""
}

func extractStringArrayField(block, fieldName string) []string {
	rx := regexp.MustCompile(`"([^"]*)"`)
	idx := strings.Index(block, fieldName)
	if idx == -1 {
		return nil
	}
	arrStart := strings.Index(block[idx:], "[")
	if arrStart == -1 {
		return nil
	}
	arrStart += idx
	arrEnd := findMatchingBracket(block, arrStart)
	if arrEnd == -1 {
		return nil
	}
	arrContent := block[arrStart : arrEnd+1]
	matches := rx.FindAllStringSubmatch(arrContent, -1)
	var result []string
	for _, m := range matches {
		result = append(result, m[1])
	}
	return result
}

// composeLayout resolves layout = "..." from frontmatter or discovers
// nested layout files, and inserts the page HTML into <nguyen-slot />.
func composeLayout(f *parser.File, pageHTML string, layouts []router.LayoutInfo) string {
	// Explicit layout reference takes priority
	if m := layoutRx.FindStringSubmatch(f.GoCode); len(m) >= 2 {
		return composeWithLayoutRef(m[1], f, pageHTML)
	}

	// Auto-discover nested layouts from deepest to shallowest
	var applicable []router.LayoutInfo
	pageDir := filepath.Dir(f.Path)
	for _, l := range layouts {
		lDir := filepath.ToSlash(l.DirPath)
		if strings.HasPrefix(pageDir, lDir) || lDir == "." {
			applicable = append(applicable, l)
		}
	}
	if len(applicable) == 0 {
		return pageHTML
	}

	// Compose from deepest (most specific) to shallowest (root).
	// Each slot insertion is wrapped with data-nguyen-outlet for SPA navigation.
	result := pageHTML
	for i := len(applicable) - 1; i >= 0; i-- {
		layoutFile, err := parser.Parse(applicable[i].FilePath)
		if err != nil {
			continue
		}
		depth := i
		wrappedResult := `<div data-nguyen-outlet="` + Itoa(depth) + `">` + result + `</div>`
		result = slotRx.ReplaceAllString(layoutFile.HTMLTemplate, wrappedResult)
		result = replaceNamedSlots(f, layoutFile, result)
	}
	return result
}

func composeWithLayoutRef(layoutRef string, f *parser.File, pageHTML string) string {
	layoutRef += ".gox"
	layoutFile, err := parser.Parse(layoutRef)
	if err != nil {
		pageDir := filepath.Dir(f.Path)
		layoutFile, err = parser.Parse(filepath.Join(pageDir, layoutRef))
		if err != nil {
			return pageHTML // layout not found
		}
	}
	return slotRx.ReplaceAllString(layoutFile.HTMLTemplate, pageHTML)
}

// replaceNamedSlots moves content with slot="name" into matching <nguyen-slot name="..." />
func replaceNamedSlots(f *parser.File, layout *parser.File, pageHTML string) string {
	// Extract named slot content from page HTML
	slotContentRx := regexp.MustCompile(`<(\w+)\b[^>]*\bslot="([^"]+)"[^>]*>(.*?)</\1>`)
	matches := slotContentRx.FindAllStringSubmatch(pageHTML, -1)
	slotContents := make(map[string]string)
	for _, m := range matches {
		name := m[2]
		content := m[3]
		slotContents[name] = content
	}

	// Replace named slots in layout
	result := slotNamedRx.ReplaceAllStringFunc(layout.HTMLTemplate, func(match string) string {
		name := ""
		if m := slotNamedRx.FindStringSubmatch(match); len(m) > 1 {
			name = m[1]
		}
		if content, ok := slotContents[name]; ok {
			return content
		}
		return match // no override
	})
	return result
}

// htmlEscape escapes HTML special characters to prevent XSS.
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// injectHydrationMarkers adds event handler attributes and injects the
// vanilla JS hydration script for SSR mode (no WASM required).
// State variable spans (data-nguyen-text) are already handled in RenderSSR.
func injectHydrationMarkers(html string, state map[string]string) string {
	// Replace @event="handler()" with data-nguyen-{event} attributes
	html = eventAttrRx.ReplaceAllStringFunc(html, func(match string) string {
		m := eventAttrRx.FindStringSubmatch(match)
		if len(m) < 3 {
			return match
		}
		eventName := m[1]
		handlerExpr := m[2]
		// Remove () from handler name if present
		handlerName := strings.TrimSuffix(handlerExpr, "()")
		return fmt.Sprintf(` data-nguyen-%s="%s"`, eventName, handlerName)
	})

	// Inject JS hydration script before </body> (after all body content is rendered)
	hydrationJS := buildHydrationJS()
	if idx := strings.LastIndex(html, "</body>"); idx != -1 {
		html = html[:idx] + hydrationJS + html[idx:]
	}

	// Wrap body content with hydration container if not present
	if strings.Contains(html, "<body") && !strings.Contains(html, "data-nguyen-hydrate") {
		html = strings.Replace(html, "<body", `<body data-nguyen-hydrate`, 1)
	}

	return html
}

// buildHydrationJS generates vanilla JS for reactive state hydration in SSR mode.
// Transpiles UseGlobalState + handlers from Go code to equivalent JS.
func buildHydrationJS() string {
	return `
<script>
(function() {
  'use strict';

  // SSR hydration: vanilla JS reactive state (no WASM required)
  // Works with data-nguyen-* attributes generated by SSR hydration markers.

  var _state = {};
  var _listeners = {};

  // Expose core primitives globally so apps can register their own handlers
  window.__nguyenState = _state;
  window.__nguyenSetState = _setState;

  window.__nguyenHandlers = window.__nguyenHandlers || {};

  // --- State initialization: read SSR-rendered data-nguyen-text values ---
  (function initState() {
    document.querySelectorAll('[data-nguyen-text]').forEach(function(el) {
      var key = el.getAttribute('data-nguyen-text');
      _state[key] = el.textContent;
    });
  })();

  // --- State helpers ---
  function _getState(key) { return _state[key]; }
  function _setState(key, val) {
    _state[key] = val;
    // Update text nodes
    document.querySelectorAll('[data-nguyen-text=' + JSON.stringify(key) + ']').forEach(function(el) {
      el.textContent = val;
    });
    // Notify listeners
    (_listeners[key] || []).forEach(function(fn) { fn(val); });
  }

  function _useState(key, initial) {
    if (!(key in _state)) { _state[key] = initial; }
    return [_getState(key), function(v) { _setState(key, v); }];
  }

  function _effect(key, fn) {
    _listeners[key] = _listeners[key] || [];
    _listeners[key].push(fn);
    fn();
  }

  // --- Auto-detect handlers from data-nguyen-{event} attributes ---
  function _bindEvents() {
    var evTypes = ['click', 'input', 'change', 'submit', 'keydown', 'keyup'];
    evTypes.forEach(function(evType) {
      document.addEventListener(evType, function(e) {
        var el = e.target.closest('[data-nguyen-' + evType + ']');
        if (!el) return;
        var name = el.getAttribute('data-nguyen-' + evType);
        if (!name) return;
        var fn = window.__nguyenHandlers[name] || window.__nguyenHandlers[name + '_' + evType];
        if (fn) {
          e.preventDefault();
          fn(e);
        }
      });
    });
  }

  // NOTE: App-specific handlers (toggleTheme, increment, decrement, reset)
  // should be defined by the application in its own <script> tags.
  // The framework only provides the reactivity primitives and event delegation.

  // --- Initialize: bind events when DOM is ready ---
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', _bindEvents);
  } else {
    _bindEvents();
  }
})();
</script>`
}

func findMatchingBrace(s string, start int) int {
	depth := 0
	for i := start; i < len(s); i++ {
		if s[i] == '{' {
			depth++
		} else if s[i] == '}' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func findMatchingBracket(s string, start int) int {
	depth := 0
	for i := start; i < len(s); i++ {
		if s[i] == '[' {
			depth++
		} else if s[i] == ']' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// extractState parses hook-style state declarations from Go code
func extractState(goCode string, state map[string]string) {
	stateRx := regexp.MustCompile(`(\w+)(?:,\s*\w+)?\s*:=\s*(?:core\.)?Use(?:Global)?State\("(\w+)",\s*([^)]+)\)`)
	matches := stateRx.FindAllStringSubmatch(goCode, -1)
	for _, m := range matches {
		key := m[2]
		val := m[3]
		val = strings.TrimSpace(val)
		val = strings.Trim(val, `"`)
		state[key] = val
	}
}

// buildMetaTagHTML builds the meta tag string for injection.
// All user-controlled strings (title, description, og:*) are HTML-escaped
// to prevent stored XSS via frontmatter metadata.
func buildMetaTagHTML(result *Result) string {
	var sb strings.Builder

	sb.WriteString("\n    <!-- Generated by Nguyen.go SSR -->\n")
	sb.WriteString(fmt.Sprintf(`    <meta name="X-Powered-By" content="Nguyen.go V%s">`+"\n", htmlEscape(result.Version)))

	meta := result.Meta
	if title, ok := meta["title"]; ok {
		sb.WriteString(fmt.Sprintf("    <title>%s</title>\n", htmlEscape(title)))
	}
	if desc, ok := meta["description"]; ok {
		sb.WriteString(fmt.Sprintf(`    <meta name="description" content="%s">`+"\n", htmlEscape(desc)))
	}
	for k, v := range meta {
		if k == "title" || k == "description" {
			continue
		}
		sb.WriteString(fmt.Sprintf(`    <meta property="%s" content="%s">`+"\n", htmlEscape(k), htmlEscape(v)))
	}

	// GEO-specific tags
	if result.GEO != nil {
		if result.GEO.DatePublished != "" {
			sb.WriteString(fmt.Sprintf(`    <meta property="article:published_time" content="%s">`+"\n", htmlEscape(result.GEO.DatePublished)))
		}
		if result.GEO.DateModified != "" {
			sb.WriteString(fmt.Sprintf(`    <meta property="article:modified_time" content="%s">`+"\n", htmlEscape(result.GEO.DateModified)))
		}
	}

	return sb.String()
}

// injectMetaTags adds <title>, <meta>, JSON-LD, and GEO tags into <head>
// of a full HTML document. Returns unchanged if no <head> or <body> tag found.
func injectMetaTags(html string, result *Result) string {
	sb := buildMetaTagHTML(result)

	// Insert before </head>
	if idx := strings.Index(html, "</head>"); idx != -1 {
		return html[:idx] + sb + html[idx:]
	}

	// No </head> found — this is likely a body fragment. Do not inject raw meta tags.
	return html
}

// InjectMetaTags adds SSR meta tags into a full HTML shell that already contains
// <head> and <body>. It replaces <nguyen-head /> if present, otherwise inserts before </head>.
func InjectMetaTags(html string, result *Result) string {
	meta := buildMetaTagHTML(result)

	// Replace <nguyen-head /> placeholder if present
	if idx := strings.Index(html, "<nguyen-head"); idx != -1 {
		end := strings.Index(html[idx:], ">")
		if end != -1 {
			// Handle both <nguyen-head /> and <nguyen-head/>
			return html[:idx] + meta + html[idx+end+1:]
		}
	}

	// Insert before </head>
	if idx := strings.Index(html, "</head>"); idx != -1 {
		return html[:idx] + meta + html[idx:]
	}

	return html
}

// WrapHTML wraps a body HTML fragment with the standard Nguyen.go HTML shell,
// injecting meta tags into <head>. Use this when the page content does NOT
// already have a complete HTML document structure (no layout used).
func WrapHTML(bodyHTML string, result *Result) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <link rel="stylesheet" href="/styles/output.css">
    <script src="/bridge/nguyen_bridge.js" defer></script>
` + result.MetaTags + `
</head>
<body>
    <div id="app">
` + bodyHTML + `
    </div>
</body>
</html>`
}
