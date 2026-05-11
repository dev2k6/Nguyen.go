//go:build js && wasm

package core

import "syscall/js"

// SetJSONLD injects a JSON-LD structured data script into <head>.
// The schema parameter is a JSON string of the schema.org data.
func SetJSONLD(schema string) {
	doc := js.Global().Get("document")
	head := doc.Get("head")

	// Remove any existing JSON-LD
	scripts := head.Call("querySelectorAll", "script[type='application/ld+json']")
	for i := 0; i < scripts.Get("length").Int(); i++ {
		scripts.Call("item", i).Call("remove")
	}

	// Create new JSON-LD script
	script := doc.Call("createElement", "script")
	script.Call("setAttribute", "type", "application/ld+json")
	script.Set("textContent", schema)
	head.Call("appendChild", script)
}

// SetDatePublished sets the datePublished meta tag for content freshness
func SetDatePublished(date string) {
	setMetaTag(js.Global().Get("document"), "property", "article:published_time", date)
}

// SetDateModified sets the dateModified meta tag
func SetDateModified(date string) {
	setMetaTag(js.Global().Get("document"), "property", "article:modified_time", date)
}

// SetSpeakableSpec marks CSS selectors as speakable for AI voice assistants
func SetSpeakableSpec(selectors []string) {
	schema := "[{"
	schema += `"@context":"https://schema.org",`
	schema += `"@type":"SpeakableSpecification",`
	schema += `"cssSelector":[`
	for i, sel := range selectors {
		if i > 0 {
			schema += ","
		}
		schema += `"` + sel + `"`
	}
	schema += "]}]"
	// Merge with existing JSON-LD or inject new
	doc := js.Global().Get("document")
	head := doc.Get("head")
	script := doc.Call("createElement", "script")
	script.Call("setAttribute", "type", "application/ld+json")
	script.Set("textContent", schema)
	head.Call("appendChild", script)
}

// BreadcrumbItem represents a breadcrumb step
type BreadcrumbItem struct {
	Name string
	URL  string
}

// SetBreadcrumb sets breadcrumb structured data for AI crawlers
func SetBreadcrumb(items []BreadcrumbItem) {
	if len(items) == 0 {
		return
	}

	var schema string
	schema += `{"@context":"https://schema.org","@type":"BreadcrumbList","itemListElement":[`
	for i, bc := range items {
		if i > 0 {
			schema += ","
		}
		schema += `{"@type":"ListItem","position":`
		schema += itoa(i + 1)
		schema += `,"name":"` + bc.Name + `","item":"` + bc.URL + `"}`
	}
	schema += "]}"

	doc := js.Global().Get("document")
	head := doc.Get("head")
	script := doc.Call("createElement", "script")
	script.Call("setAttribute", "type", "application/ld+json")
	script.Set("textContent", schema)
	head.Call("appendChild", script)
}

// itoa converts int to string without fmt import
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := "0123456789"
	result := ""
	for n > 0 {
		result = string(digits[n%10]) + result
		n /= 10
	}
	return result
}
