//go:build js && wasm

package core

import (
	"syscall/js"
)

// NguyenTransition wraps children with CSS enter/leave animations.
// It adds/removes CSS classes to trigger transitions on mount/unmount.
//
// Usage in .nguyen template:
//
//	<nguyen-transition name="fade">
//	    <div>Content</div>
//	</nguyen-transition>
//
// CSS required:
//
//	.nguyen-fade-enter        { opacity: 0; }
//	.nguyen-fade-enter-active { opacity: 1; transition: opacity 300ms; }
//	.nguyen-fade-leave        { opacity: 1; }
//	.nguyen-fade-leave-active { opacity: 0; transition: opacity 300ms; }
func NguyenTransition(name string, children ...*VNode) *VNode {
	// On mount, add enter class then enter-active after a tick.
	UseEffect(func() interface{} {
		// CSS transition class toggling is done via JS bridge
		js.Global().Call("__nguyenTransitionEnter", name)
		return func() {
			js.Global().Call("__nguyenTransitionLeave", name)
		}
	}, []interface{}{name})

	return H("div", Attr{
		"data-nguyen-transition": name,
	}, func() []interface{} {
		var out []interface{}
		for _, c := range children {
			out = append(out, c)
		}
		return out
	}())
}

// TransitionGroup manages enter/leave animations for a list of items
// that are reordered or added/removed.
func TransitionGroup(name string, children ...*VNode) *VNode {
	return H("div", Attr{
		"data-nguyen-transition-group": name,
	}, func() []interface{} {
		var out []interface{}
		for _, c := range children {
			out = append(out, c)
		}
		return out
	}())
}
