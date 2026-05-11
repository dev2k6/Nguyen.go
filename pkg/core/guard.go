//go:build js && wasm

package core

import "syscall/js"

// RouteGuard defines a predicate that decides whether navigation is allowed.
// If Allowed returns false, the user is redirected to RedirectTo.
type RouteGuard struct {
	Name       string
	Allowed    func(path string) bool
	RedirectTo string
}

var registeredGuards = make(map[string]*RouteGuard)

// RegisterGuard registers a named route guard for use in frontmatter.
//
// Usage:
//
//	core.RegisterGuard("auth", &core.RouteGuard{
//	    Allowed: func(path string) bool {
//	        return token != ""
//	    },
//	    RedirectTo: "/login",
//	})
func RegisterGuard(name string, guard *RouteGuard) {
	registeredGuards[name] = guard
}

// GetGuard retrieves a previously registered guard by name.
func GetGuard(name string) *RouteGuard {
	return registeredGuards[name]
}

// CheckGuard evaluates a guard for the current path.
// Returns true if allowed, false if redirect is needed.
func CheckGuard(name string, path string) (bool, string) {
	g, ok := registeredGuards[name]
	if !ok {
		return true, ""
	}
	if g.Allowed(path) {
		return true, ""
	}
	return false, g.RedirectTo
}

// UseRouteGuard runs a guard check on mount and intercepts client-side navigation.
// If the guard fails, automatically calls Navigate to the redirect target.
//
// Usage:
//
//	func Render() *core.VNode {
//	    core.UseRouteGuard("auth", "/login")
//	    // ... rest of render
//	}
func UseRouteGuard(guardName, redirectTo string) {
	allowed, _ := CheckGuard(guardName, CurrentPath())
	if !allowed {
		Navigate(redirectTo)
		return
	}

	// Re-check on every navigation
	UseEffect(func() interface{} {
		checkNav := func(path string) {
			ok, _ := CheckGuard(guardName, path)
			if !ok {
				Navigate(redirectTo)
			}
		}
		OnNavigate(checkNav)
		return nil
	}, []interface{}{guardName, redirectTo})
}

// ActiveLink updates a link element's class when its href matches the current route.
// Call this in a component that renders links.
//
// Usage:
//
//	func Render() *core.VNode {
//	    core.ActiveLink("/about", "active", true)
//	    return core.H("a", core.Attr{"href": "/about", "class": "nav-link"})
//	}
func ActiveLink(href, activeClass string, exact bool) {
	UseEffect(func() interface{} {
		updateClass := func() {
			isActive := IsActivePath(href, exact)
			doc := js.Global().Get("document")
			links := doc.Call("querySelectorAll", "[href='"+href+"']")
			for i := 0; i < links.Get("length").Int(); i++ {
				el := links.Call("item", i)
				cls := el.Get("className").String()
				if isActive {
					if !containsClass(cls, activeClass) {
						el.Set("className", cls+" "+activeClass)
					}
				} else {
					el.Set("className", removeClass(cls, activeClass))
				}
			}
		}
		updateClass()
		OnNavigate(func(path string) {
			updateClass()
		})
		return nil
	}, []interface{}{href, activeClass, exact})
}

func containsClass(classes, cls string) bool {
	for _, c := range splitClass(classes) {
		if c == cls {
			return true
		}
	}
	return false
}

func removeClass(classes, cls string) string {
	var out []string
	for _, c := range splitClass(classes) {
		if c != cls {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return ""
	}
	return join(out, " ")
}

func splitClass(s string) []string {
	var parts []string
	for _, p := range split(s, " ") {
		p = trim(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func trim(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func split(s, sep string) []string {
	var parts []string
	for {
		i := 0
		for i < len(s) {
			if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
				parts = append(parts, s[:i])
				s = s[i+len(sep):]
				break
			}
			i++
		}
		if i == len(s) {
			parts = append(parts, s)
			break
		}
	}
	return parts
}

func join(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}
