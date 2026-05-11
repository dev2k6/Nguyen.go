//go:build js && wasm

package core

import (
	"strings"
	"syscall/js"
)

var (
	navigateCallbacks []func(path string)
	currentPath       string
)

// Navigate pushes a new URL to the browser history and triggers a client-side navigation.
// Called by <nguyen-link> when clicked. No full page reload.
func Navigate(path string) {
	if path == currentPath {
		return
	}
	currentPath = path
	callbacks := make([]func(string), len(navigateCallbacks))
	copy(callbacks, navigateCallbacks)

	// Update browser URL without reload
	history := js.Global().Get("history")
	history.Call("pushState", nil, "", path)

	// Notify listeners
	for _, cb := range callbacks {
		cb(path)
	}
}

// OnNavigate registers a callback that fires on every client-side navigation.
// Also fires on browser back/forward (popstate events from the bridge).
func OnNavigate(callback func(path string)) {
	navigateCallbacks = append(navigateCallbacks, callback)
}

// CurrentPath returns the current route path
func CurrentPath() string {
	if currentPath != "" {
		return currentPath
	}
	loc := js.Global().Get("location")
	currentPath = loc.Get("pathname").String()
	return currentPath
}

// CurrentSearch returns the current URL query string (without leading ?)
func CurrentSearch() string {
	loc := js.Global().Get("location")
	return loc.Get("search").String()
}

// IsActivePath returns true if the given path matches the current route.
// If exact is true, requires full equality; otherwise prefix match.
func IsActivePath(path string, exact bool) bool {
	current := CurrentPath()
	if exact {
		return path == current
	}
	return strings.HasPrefix(current, path)
}

// UseSearchParams returns parsed query parameters from the current URL.
// Re-renders the calling component when navigation changes.
func UseSearchParams() map[string]string {
	search := CurrentSearch()
	if len(search) > 0 && search[0] == '?' {
		search = search[1:]
	}

	result := make(map[string]string)
	if search == "" {
		return result
	}

	pairs := strings.Split(search, "&")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}

	// Force re-render on navigation by using a state hook behind the scenes.
	// This ensures the component re-renders when URL query changes.
	_, setNav := UseState(nil)
	_ = UseCallback(func() {
		// no-op — just reserving a hook slot for navigation tracking
	}, []interface{}{})
	OnNavigate(func(path string) {
		setNav(path)
	})

	return result
}

// handlePopState is called from JS bridge when browser back/forward is triggered
func handlePopState(this js.Value, args []js.Value) interface{} {
	loc := js.Global().Get("location")
	path := loc.Get("pathname").String()

	currentPath = path
	callbacks := make([]func(string), len(navigateCallbacks))
	copy(callbacks, navigateCallbacks)

	for _, cb := range callbacks {
		cb(path)
	}
	return nil
}

func init() {
	// Register popstate handler
	js.Global().Get("window").Call("addEventListener", "popstate",
		js.FuncOf(handlePopState))
}
