package render

import "strings"

// HTMLString marks a string as pre-escaped so subsequent render stages
// won't double-escape it. Mirrors Astro's HTMLString + markHTMLString
// pattern (escape.ts:34-55).
//
// Why: server-side rendering frequently concatenates user content with
// framework-generated markup (meta tags, hydration scripts, JSON-LD).
// Without a marker, each layer re-escapes the HTML and the output looks
// like "&amp;lt;div&amp;gt;". Tagging safe strings lets Escape() short-circuit.
type HTMLString struct {
	safe string
}

// MarkSafe constructs an HTMLString from a trusted source (framework-
// generated markup, already-escaped user content, etc.). Pass the result
// to Escape() or embed directly in template output.
func MarkSafe(s string) HTMLString {
	return HTMLString{safe: s}
}

// String implements fmt.Stringer.
func (h HTMLString) String() string { return h.safe }

// Raw returns the underlying value without extra processing.
func (h HTMLString) Raw() string { return h.safe }

// Escape HTML-escapes a raw string. HTMLString values pass through unchanged,
// mirroring Astro's fast path: an already-escaped template fragment is never
// re-escaped when injected into a parent template.
func Escape(v interface{}) string {
	switch s := v.(type) {
	case HTMLString:
		return s.safe
	case string:
		return htmlEscape(s)
	case nil:
		return ""
	default:
		// Fallback for Stringer-like values: do not escape, caller owns safety.
		return ""
	}
}

// EscapeAttr escapes a value for embedding into an HTML attribute.
// Quotes, angle brackets, and ampersands are always escaped even if
// the caller marked the value as safe, because attribute context is
// more permissive than element context and double-escaping a quote is
// safer than leaking out of an attribute value.
func EscapeAttr(v interface{}) string {
	s := ""
	switch vv := v.(type) {
	case HTMLString:
		s = vv.safe
	case string:
		s = vv
	default:
		return ""
	}
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
