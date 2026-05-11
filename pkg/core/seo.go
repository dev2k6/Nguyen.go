//go:build js && wasm

package core

import "syscall/js"

// SetMeta updates the browser's <head> with title and meta tags at runtime.
// For SSR, this is injected server-side by the render package instead.
func SetMeta(title, description string) {
	doc := js.Global().Get("document")

	// Update <title>
	if title != "" {
		doc.Set("title", title)
	}

	// Update or create <meta name="description">
	if description != "" {
		setMetaTag(doc, "name", "description", description)
	}
}

// SetOGTag sets an Open Graph meta tag
func SetOGTag(property, content string) {
	doc := js.Global().Get("document")
	setMetaTag(doc, "property", property, content)
}

// SetCanonical sets the canonical URL
func SetCanonical(url string) {
	doc := js.Global().Get("document")
	head := doc.Get("head")

	// Remove existing canonical
	links := head.Call("querySelectorAll", `link[rel="canonical"]`)
	for i := 0; i < links.Get("length").Int(); i++ {
		links.Call("item", i).Call("remove")
	}

	if url != "" {
		link := doc.Call("createElement", "link")
		link.Call("setAttribute", "rel", "canonical")
		link.Call("setAttribute", "href", url)
		head.Call("appendChild", link)
	}
}

func setMetaTag(doc js.Value, attrName, attrValue, content string) {
	head := doc.Get("head")
	selector := `meta[` + attrName + `="` + attrValue + `"]`
	el := head.Call("querySelector", selector)
	if el.IsNull() {
		el = doc.Call("createElement", "meta")
		el.Call("setAttribute", attrName, attrValue)
		head.Call("appendChild", el)
	}
	el.Call("setAttribute", "content", content)
}
