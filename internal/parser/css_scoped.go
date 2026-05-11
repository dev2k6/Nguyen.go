package parser

import (
	"crypto/sha256"
	"fmt"
	"regexp"
)

var scopedStyleRx = regexp.MustCompile(`<style\s+scoped\s*>([\s\S]*?)</style>`)

// ScopedCSS holds the result of processing scoped CSS
type ScopedCSS struct {
	Hash string            // unique hash for this component
	CSS  string            // rewritten CSS with hashed selectors
	Map  map[string]string // original class → hashed class
}

// ProcessScopedCSS finds <style scoped> blocks in HTML and generates unique hashes.
// Returns the modified HTML with <style scoped> → <style data-v-{hash}> and
// a ScopedCSS result for each scoped style block found.
func ProcessScopedCSS(filepath string, html string) (string, map[string]*ScopedCSS) {
	results := make(map[string]*ScopedCSS)

	result := scopedStyleRx.ReplaceAllStringFunc(html, func(match string) string {
		// Generate hash from file path
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(filepath)))[:6]
		scoped := &ScopedCSS{Hash: hash, Map: make(map[string]string)}

		// Extract CSS content
		sub := scopedStyleRx.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		css := sub[1]

		// Replace class-based selectors with hashed versions
		scoped.CSS = scopeCSS(css, hash, scoped.Map)

		results[filepath] = scoped

		// Rewrite the <style> tag
		return fmt.Sprintf(`<style data-v-%s>
%s</style>`, hash, scoped.CSS)
	})

	// Apply hashed class names to HTML elements
	for _, scoped := range results {
		for original, hashed := range scoped.Map {
			// Replace class="original" → class="original hashed"
			classRx := regexp.MustCompile(`class="` + regexp.QuoteMeta(original) + `"`)
			result = classRx.ReplaceAllString(result,
				fmt.Sprintf(`class="%s %s"`, original, hashed))
		}
	}

	return result, results
}

// scopeCSS rewrites CSS selectors to include a data-v-{hash} scope constraint.
func scopeCSS(css string, hash string, classMap map[string]string) string {
	// Match class selectors: .classname
	classRx := regexp.MustCompile(`\.([a-zA-Z_][\w-]*)`)

	seen := make(map[string]bool)

	result := classRx.ReplaceAllStringFunc(css, func(match string) string {
		name := match[1:] // strip leading .
		if seen[name] {
			return match
		}
		seen[name] = true

		hashed := name + "-" + hash
		classMap[name] = hashed

		// Append data-v constraint to the selector
		return fmt.Sprintf(".%s", hashed)
	})

	return result
}
