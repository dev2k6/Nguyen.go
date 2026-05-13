package render

import (
	"regexp"
	"strings"
)

// Fragment processing for <nguyen-fragment> elements.
// Fragments render their children without a wrapper element,
// useful for returning multiple sibling elements from a component.
//
// Usage in .gox template:
//
//	<nguyen-fragment>
//	    <li>Item 1</li>
//	    <li>Item 2</li>
//	</nguyen-fragment>
//
// Renders as:
//
//	<li>Item 1</li>
//	<li>Item 2</li>

var fragmentOpenRx = regexp.MustCompile(`<nguyen-fragment[^>]*>`)
var fragmentCloseRx = regexp.MustCompile(`</nguyen-fragment>`)

// ProcessFragments removes <nguyen-fragment> wrapper tags from HTML,
// leaving only their inner content. This enables components to return
// multiple root elements without an artificial wrapper div.
func ProcessFragments(html string) string {
	html = fragmentOpenRx.ReplaceAllString(html, "")
	html = fragmentCloseRx.ReplaceAllString(html, "")
	return html
}

// Conditional rendering helpers for SSR templates.
// These process <nguyen-show> and <nguyen-hide> directives.
//
// <nguyen-show when="condition">...</nguyen-show>
// <nguyen-hide when="condition">...</nguyen-hide>

var showOpenRx = regexp.MustCompile(`<nguyen-show\s+when="([^"]*)"[^>]*>`)
var showCloseRx = regexp.MustCompile(`</nguyen-show>`)
var hideOpenRx = regexp.MustCompile(`<nguyen-hide\s+when="([^"]*)"[^>]*>`)
var hideCloseRx = regexp.MustCompile(`</nguyen-hide>`)

// ProcessConditionals evaluates <nguyen-show> and <nguyen-hide> directives
// against the provided state map. Truthy values: non-empty, non-"0", non-"false".
func ProcessConditionals(html string, state map[string]string) string {
	// Process <nguyen-show when="key">
	html = processDirective(html, showOpenRx, showCloseRx, state, false)
	// Process <nguyen-hide when="key">
	html = processDirective(html, hideOpenRx, hideCloseRx, state, true)
	return html
}

func processDirective(html string, openRx, closeRx *regexp.Regexp, state map[string]string, invert bool) string {
	for {
		loc := openRx.FindStringSubmatchIndex(html)
		if loc == nil {
			break
		}

		openStart := loc[0]
		openEnd := loc[1]
		condKey := html[loc[2]:loc[3]]

		closeIdx := closeRx.FindStringIndex(html[openEnd:])
		if closeIdx == nil {
			break
		}

		closeStart := openEnd + closeIdx[0]
		closeEnd := openEnd + closeIdx[1]

		innerContent := html[openEnd:closeStart]
		condValue := isTruthy(state[condKey])

		shouldShow := condValue
		if invert {
			shouldShow = !condValue
		}

		var replacement string
		if shouldShow {
			replacement = innerContent
		}

		html = html[:openStart] + replacement + html[closeEnd:]
	}
	return html
}

func isTruthy(val string) bool {
	val = strings.TrimSpace(val)
	return val != "" && val != "0" && val != "false" && val != "nil"
}
