package optimizer

import (
	"fmt"
	"regexp"
	"strings"
)

// CriticalCSS holds extracted critical and deferred CSS rules
type CriticalCSSResult struct {
	Critical string // CSS rules for above-fold elements (inline in <head>)
	Deferred string // Remaining CSS rules (load async)
}

var (
	classRx    = regexp.MustCompile(`class="([^"]+)"`)
	idRx       = regexp.MustCompile(`id="([^"]+)"`)
	tagRx      = regexp.MustCompile(`<(\w+)`)
	selectorRx = regexp.MustCompile(`([.#]?[a-zA-Z_][\w-]*)`)
)

// ExtractCritical extracts CSS rules matching elements in the above-the-fold HTML.
// criticalBudgetKB limits the inline CSS size (default 14KB per Chrome's recommendation).
// Returns critical CSS for inline and deferred CSS for async loading.
func ExtractCritical(html string, css string, criticalBudgetKB int) *CriticalCSSResult {
	if criticalBudgetKB <= 0 {
		criticalBudgetKB = 14
	}

	// Extract all class names, IDs, and tag names from HTML
	usedClasses := extractUsedClasses(html)
	usedIDs := extractUsedIDs(html)
	usedTags := extractUsedTags(html)

	// Split CSS into rules
	rules := splitCSSRules(css)

	var critical []string
	var deferred []string
	criticalSize := 0
	budgetBytes := criticalBudgetKB * 1024

	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}

		selector := extractSelector(rule)
		if selector == "" {
			deferred = append(deferred, rule)
			continue
		}

		if matchesUsed(selector, usedClasses, usedIDs, usedTags) {
			// Add to critical if within budget
			if criticalSize < budgetBytes {
				critical = append(critical, rule)
				criticalSize += len(rule)
			} else {
				deferred = append(deferred, rule)
			}
		} else {
			deferred = append(deferred, rule)
		}
	}

	// If no critical rules found, keep first 5 rules as critical (heuristic)
	if len(critical) == 0 && len(deferred) > 5 {
		critical = deferred[:5]
		deferred = deferred[5:]
	}

	return &CriticalCSSResult{
		Critical: strings.Join(critical, "\n"),
		Deferred: strings.Join(deferred, "\n"),
	}
}

// BuildCriticalStyleTag wraps critical CSS in a <style> tag
func BuildCriticalStyleTag(criticalCSS string) string {
	if criticalCSS == "" {
		return ""
	}
	return fmt.Sprintf("<style>\n%s\n</style>", criticalCSS)
}

// BuildDeferredCSSLink creates a deferred CSS load pattern
func BuildDeferredCSSLink(href string) string {
	return fmt.Sprintf(`<link rel="preload" as="style" href="%s" onload="this.onload=null;this.rel='stylesheet'">`+
		`<noscript><link rel="stylesheet" href="%s"></noscript>`, href, href)
}

// extractUsedClasses finds all class names in HTML
func extractUsedClasses(html string) map[string]bool {
	used := make(map[string]bool)
	matches := classRx.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		for _, cls := range strings.Fields(m[1]) {
			used[cls] = true
		}
	}
	return used
}

// extractUsedIDs finds all IDs in HTML
func extractUsedIDs(html string) map[string]bool {
	used := make(map[string]bool)
	matches := idRx.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		used[m[1]] = true
	}
	return used
}

// extractUsedTags finds all tag names in HTML
func extractUsedTags(html string) map[string]bool {
	used := make(map[string]bool)
	matches := tagRx.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		tag := strings.ToLower(m[1])
		used[tag] = true
	}
	return used
}

// splitCSSRules splits a CSS string into individual rules
func splitCSSRules(css string) []string {
	css = strings.ReplaceAll(css, "\r\n", "\n")
	var rules []string
	var current strings.Builder
	depth := 0

	for _, ch := range css {
		current.WriteRune(ch)
		switch ch {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				rules = append(rules, current.String())
				current.Reset()
			}
		}
	}
	// Any remaining content
	if current.Len() > 0 {
		rules = append(rules, current.String())
	}
	return rules
}

// extractSelector gets the CSS selector from a rule (before the {)
func extractSelector(rule string) string {
	idx := strings.Index(rule, "{")
	if idx == -1 {
		return ""
	}
	return strings.TrimSpace(rule[:idx])
}

// matchesUsed checks if a CSS selector matches any used classes, IDs, or tags
func matchesUsed(selector string, classes, ids, tags map[string]bool) bool {
	// Check class selectors: .classname
	matches := selectorRx.FindAllString(selector, -1)
	for _, part := range matches {
		if strings.HasPrefix(part, ".") {
			if classes[part[1:]] {
				return true
			}
		} else if strings.HasPrefix(part, "#") {
			if ids[part[1:]] {
				return true
			}
		} else {
			// Tag selector
			if tags[strings.ToLower(part)] {
				return true
			}
		}
	}
	return false
}
