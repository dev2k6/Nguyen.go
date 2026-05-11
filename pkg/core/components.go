//go:build js && wasm

package core

import (
	"fmt"
	"strings"
	"syscall/js"
)

var document js.Value

func init() {
	document = js.Global().Get("document")
}

// NguyenLink creates an <a> element that navigates client-side via History API.
// Attributes: href (required), class, id, prefetch (optional)
func NguyenLink(props map[string]string, children ...js.Value) js.Value {
	href := props["href"]
	a := CreateElement("a")
	a.Call("setAttribute", "href", href)
	if cls, ok := props["class"]; ok {
		a.Call("setAttribute", "class", cls)
	}
	if id, ok := props["id"]; ok {
		a.Call("setAttribute", "id", id)
	}

	// Append children
	for _, child := range children {
		a.Call("appendChild", child)
	}

	// Client-side navigation on click
	a.Call("addEventListener", "click", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		args[0].Call("preventDefault")
		Navigate(href)

		// Prefetch support: load page data in background
		if _, ok := props["prefetch"]; ok {
			go func() {
				js.Global().Call("fetch", href)
			}()
		}
		return nil
	}))

	return a
}

// NguyenSlot represents a placeholder for child content in layouts.
// In SSR mode, this is replaced with the actual page content.
// In CSR mode, this is the mount point for the current page component.
func NguyenSlot() js.Value {
	div := CreateElement("div")
	div.Call("setAttribute", "data-nguyen-slot", "")
	return div
}

// NguyenHead injects metadata into the document <head>.
// Called once per page render to update title and meta tags.
func NguyenHead(title, description string) {
	SetMeta(title, description)
}

// NguyenImage creates an optimized <img> element with lazy loading.
// Adds data-srcset for responsive sizes and data-blur for LQIP placeholder.
// In SSR mode, these are replaced with actual srcset and blur inline style.
func NguyenImage(src string, width, height int, alt string) js.Value {
	img := CreateElement("img")
	img.Call("setAttribute", "src", src)
	img.Call("setAttribute", "loading", "lazy")
	img.Call("setAttribute", "decoding", "async")
	if width > 0 {
		img.Call("setAttribute", "width", width)
	}
	if height > 0 {
		img.Call("setAttribute", "height", height)
	}
	if alt != "" {
		img.Call("setAttribute", "alt", alt)
	}
	// Responsive srcset: computed by bridge.js from data attributes
	// Pattern: /_nguyen/image?src=<src>&w=<width>&q=80&f=webp
	img.Call("setAttribute", "data-nguyen-img", src)

	return img
}

// NguyenImageSSR returns HTML attributes for SSR-rendered NguyenImage.
// Used by the transpiler SSR path to inject srcset and blur.
func NguyenImageSSR(src string, width, height int, alt string, srcset string, blur string) string {
	attrs := fmt.Sprintf(`src="%s" loading="lazy" decoding="async"`, src)
	if width > 0 {
		attrs += fmt.Sprintf(` width="%d"`, width)
	}
	if height > 0 {
		attrs += fmt.Sprintf(` height="%d"`, height)
	}
	if alt != "" {
		attrs += fmt.Sprintf(` alt="%s"`, alt)
	}
	if srcset != "" {
		attrs += fmt.Sprintf(` srcset="%s"`, srcset)
	}
	if blur != "" {
		attrs += fmt.Sprintf(` style="background-image:url('%s');background-size:cover"`, blur)
	}
	return attrs
}

// NguyenImageSrcSet builds a srcset string from width→URL map.
func NguyenImageSrcSet(srcSet map[int]string) string {
	var parts []string
	// Sorted by width
	sizes := []int{640, 750, 1080, 1920}
	for _, w := range sizes {
		if url, ok := srcSet[w]; ok {
			parts = append(parts, fmt.Sprintf("%s %dw", url, w))
		}
	}
	return strings.Join(parts, ", ")
}
