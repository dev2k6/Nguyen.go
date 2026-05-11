//go:build js && wasm

package core

import (
	"encoding/json"
	"strings"
	"syscall/js"
)

// HydrationDirective controls when and how a component becomes interactive.
// Modeled on Astro's client:* directives (hydration.ts:27-115).
//
// Use Island() to wrap a VNode for partial hydration; the reconciler
// renders an SSR placeholder and defers the WASM mount until the
// directive's trigger fires in the browser.
type HydrationDirective int

const (
	// HydrateNone: fully static — no client-side JS (default).
	HydrateNone HydrationDirective = iota
	// HydrateLoad: mount as soon as the page loads (Astro: client:load).
	HydrateLoad
	// HydrateIdle: mount when requestIdleCallback fires (Astro: client:idle).
	HydrateIdle
	// HydrateVisible: mount when IntersectionObserver reports visible (Astro: client:visible).
	HydrateVisible
	// HydrateMedia: mount when a media query matches (Astro: client:media).
	HydrateMedia
	// HydrateOnly: skip SSR entirely, render on client only (Astro: client:only).
	HydrateOnly
)

// String returns the directive name for use in SSR markers.
func (h HydrationDirective) String() string {
	switch h {
	case HydrateLoad:
		return "load"
	case HydrateIdle:
		return "idle"
	case HydrateVisible:
		return "visible"
	case HydrateMedia:
		return "media"
	case HydrateOnly:
		return "only"
	default:
		return "none"
	}
}

// IslandConfig carries the directive and its argument (e.g. media query string).
type IslandConfig struct {
	Directive HydrationDirective
	Value     string // media query for HydrateMedia; component name otherwise
	Name      string // component export name for client runtime lookup
	Props     Props  // serialized and embedded for client bootstrap
}

// Island wraps a child VNode with hydration metadata. The SSR output embeds
// data-nguyen-island attributes the client runtime reads to schedule mounting.
// Modeled on Astro's generateHydrateScript (hydration.ts:126-187).
func Island(cfg IslandConfig, child *VNode) *VNode {
	if child == nil {
		return nil
	}

	attrs := Attr{
		"data-nguyen-island":   cfg.Directive.String(),
		"data-nguyen-island-c": cfg.Name,
	}
	if cfg.Value != "" {
		attrs["data-nguyen-island-v"] = cfg.Value
	}
	if cfg.Props != nil {
		// Serialize props so the client runtime can bootstrap the component
		// without a second request. escapeHTML happens at render time.
		if b, err := json.Marshal(cfg.Props); err == nil {
			attrs["data-nguyen-island-p"] = string(b)
		}
	}

	// Wrap the child in a <nguyen-island> custom element so the client
	// runtime can addEventListener without collision.
	return H("nguyen-island", attrs, child)
}

// islandRegistry stores component constructors keyed by export name.
// Client-side WASM looks up by name from data-nguyen-island-c.
var islandRegistry = make(map[string]func(Props) Component)

// RegisterIsland registers a component factory so Island() SSR markers
// can bootstrap it on the client.
func RegisterIsland(name string, factory func(Props) Component) {
	islandRegistry[name] = factory
}

// HydrateIslands scans the mounted DOM for data-nguyen-island markers and
// schedules each island for hydration based on its directive. Call once
// after bridge JS has initialized the app.
func HydrateIslands() {
	doc := js.Global().Get("document")
	nodes := doc.Call("querySelectorAll", "[data-nguyen-island]")
	length := nodes.Get("length").Int()
	for i := 0; i < length; i++ {
		el := nodes.Call("item", i)
		directive := el.Call("getAttribute", "data-nguyen-island").String()
		name := el.Call("getAttribute", "data-nguyen-island-c").String()
		value := ""
		if v := el.Call("getAttribute", "data-nguyen-island-v"); !v.IsNull() {
			value = v.String()
		}
		propsJSON := ""
		if v := el.Call("getAttribute", "data-nguyen-island-p"); !v.IsNull() {
			propsJSON = v.String()
		}
		scheduleIsland(el, directive, name, value, propsJSON)
	}
}

// scheduleIsland binds the directive's trigger to a mount callback.
func scheduleIsland(el js.Value, directive, name, value, propsJSON string) {
	mount := func() {
		factory, ok := islandRegistry[name]
		if !ok {
			return
		}
		var props Props
		if propsJSON != "" {
			_ = json.Unmarshal([]byte(propsJSON), &props)
		}
		comp := factory(props)
		inst := mountComponent(comp, props, nil)
		if inst == nil || inst.vnode == nil {
			return
		}
		// Replace the placeholder content with the real mounted tree
		el.Set("innerHTML", "")
		if !inst.vnode.domElement.IsNull() {
			el.Call("appendChild", inst.vnode.domElement)
		}
	}

	switch strings.ToLower(directive) {
	case "load":
		mount()
	case "idle":
		js.Global().Call("requestIdleCallback", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			mount()
			return nil
		}))
	case "visible":
		observerCallback := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			if len(args) < 1 {
				return nil
			}
			entries := args[0]
			if entries.Get("length").Int() == 0 {
				return nil
			}
			first := entries.Index(0)
			if first.Get("isIntersecting").Bool() {
				mount()
			}
			return nil
		})
		observerInit := map[string]interface{}{"rootMargin": "50px"}
		observer := js.Global().Get("IntersectionObserver").New(observerCallback, observerInit)
		observer.Call("observe", el)
	case "media":
		if value == "" {
			mount()
			return
		}
		mql := js.Global().Call("matchMedia", value)
		if mql.Get("matches").Bool() {
			mount()
			return
		}
		mql.Call("addEventListener", "change", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			if len(args) > 0 && args[0].Get("matches").Bool() {
				mount()
			}
			return nil
		}))
	default:
		mount()
	}
}
