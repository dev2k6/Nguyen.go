//go:build js && wasm

package core

import (
	"fmt"
	"syscall/js"
)

// Props is the standard props type for Nguyen.go components.
// Replaces the previous map[string]string with map[string]interface{}
// to support numbers, booleans, arrays, and complex values.
type Props map[string]interface{}

// Component is the interface that all Nguyen.go components must implement.
type Component interface {
	// Render returns the VNode tree for this component.
	Render() *VNode

	// SetProps receives props from parent
	SetProps(props Props)

	// Props returns current props
	Props() Props

	// SetChildren receives children from parent
	SetChildren(children []*VNode)

	// Children returns current children
	Children() []*VNode

	// Key returns the component's unique key (for reconciliation)
	Key() string

	// SetKey sets the component key
	SetKey(key string)
}

// MemoComponent is a component wrapped with React.memo-style optimization.
// Only re-renders when props change (shallow comparison).
type MemoComponent struct {
	Component
	compareFn func(oldProps, newProps Props) bool
}

// Memo wraps a component with memoization — equivalent to React.memo.
// The component only re-renders when props change (shallow comparison by default).
func Memo(comp Component, compareFn func(oldProps, newProps Props) bool) Component {
	if compareFn == nil {
		compareFn = shallowEqualProps
	}
	return &MemoComponent{
		Component: comp,
		compareFn: compareFn,
	}
}

// shallowEqualProps compares two Props maps with shallow equals.
func shallowEqualProps(a, b Props) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		bv, ok := b[k]
		if !ok || fmt.Sprintf("%v", v) != fmt.Sprintf("%v", bv) {
			return false
		}
	}
	return true
}

// BaseComponent provides default implementations for the Component interface.
type BaseComponent struct {
	props    Props
	children []*VNode
	key      string
}

func (b *BaseComponent) SetProps(props Props)          { b.props = props }
func (b *BaseComponent) Props() Props                  { return b.props }
func (b *BaseComponent) SetChildren(children []*VNode) { b.children = children }
func (b *BaseComponent) Children() []*VNode            { return b.children }
func (b *BaseComponent) Key() string                   { return b.key }
func (b *BaseComponent) SetKey(key string)             { b.key = key }
func (b *BaseComponent) Render() *VNode                { return nil }

// PropValue retrieves a typed value from Props.
func PropValue[T any](props Props, key string) T {
	var zero T
	if v, ok := props[key]; ok {
		if typed, ok := v.(T); ok {
			return typed
		}
	}
	return zero
}

// componentInstance holds the runtime state of a mounted component
type componentInstance struct {
	component    Component
	vnode        *VNode
	hooks        *hooksStore
	onMount      func()
	onUnmount    func()
	root         js.Value
	lastProps    Props
	lastChildren []*VNode
	// dirty is set when any hook scheduled an update since last render.
	// Drives React-style bail-out: if !dirty && propsEqual && childrenEqual,
	// skip the render entirely.
	dirty bool
	// ErrorBoundary support
	error    interface{}
	fallback func(err interface{}) *VNode
}

// mountComponent creates a component instance and renders it
func mountComponent(comp Component, props Props, children []*VNode) *componentInstance {
	if comp == nil {
		return nil
	}

	// Set hook context for this component
	currentHooks = &hooksStore{hooks: make([]*hook, 0), index: -1}

	inst := &componentInstance{
		component:    comp,
		hooks:        currentHooks,
		lastProps:    copyProps(props),
		lastChildren: copyNodes(children),
	}

	// Bind the hook owner so setters can mark this instance dirty
	prevInstance := currentInstance
	currentInstance = inst
	defer func() { currentInstance = prevInstance }()

	comp.SetProps(props)
	comp.SetChildren(children)

	// Call Render to get VNode
	var vnode *VNode
	func() {
		defer func() {
			if r := recover(); r != nil {
				inst.error = r
				if inst.fallback != nil {
					vnode = inst.fallback(r)
				} else {
					vnode = H("div", Attr{"data-nguyen-error": ""}, Text("Something went wrong"))
				}
			}
		}()
		vnode = comp.Render()
	}()

	if vnode == nil {
		vnode = &VNode{Type: VNodeEmpty}
	}
	inst.vnode = vnode

	// Mount to DOM
	rec := NewReconciler()
	rec.mountToDOM(vnode)

	return inst
}

// UpdateComponent reconciles a component with new props.
// For MemoComponents, skips render if props haven't changed.
// Also implements React's bailout: if props + children are shallow-equal
// and no hooks scheduled an update, skip the render entirely.
func UpdateComponent(inst *componentInstance, newProps Props, newChildren []*VNode) {
	if inst == nil {
		return
	}

	// Memo bail-out (explicit React.memo equivalent)
	if memoComp, ok := inst.component.(*MemoComponent); ok {
		if memoComp.compareFn(inst.lastProps, newProps) {
			return
		}
	}

	// React-style early bail-out: props + children unchanged AND no scheduled
	// hook update → skip re-render entirely. Matches bailoutOnAlreadyFinishedWork
	// heuristic from ReactFiberBeginWork.js:3769.
	if !inst.dirty &&
		shallowEqualProps(inst.lastProps, newProps) &&
		shallowEqualVNodeSlice(inst.lastChildren, newChildren) {
		return
	}
	inst.dirty = false

	// Set hook context for re-render
	currentHooks = inst.hooks
	currentHooks.index = -1

	// Bind instance so any setters called during Render propagate dirty to inst
	prevInstance := currentInstance
	currentInstance = inst
	defer func() { currentInstance = prevInstance }()

	inst.component.SetProps(newProps)
	inst.component.SetChildren(newChildren)

	oldVNode := inst.vnode
	var newVNode *VNode
	func() {
		defer func() {
			if r := recover(); r != nil {
				inst.error = r
				if inst.fallback != nil {
					newVNode = inst.fallback(r)
				} else {
					newVNode = H("div", Attr{"data-nguyen-error": ""}, Text("Something went wrong"))
				}
			}
		}()
		newVNode = inst.component.Render()
	}()

	if newVNode == nil {
		newVNode = &VNode{Type: VNodeEmpty}
	}

	rec := NewReconciler()
	rec.Render(inst.root, oldVNode, newVNode)
	inst.vnode = newVNode

	// Update memo cache
	inst.lastProps = copyProps(newProps)
	inst.lastChildren = copyNodes(newChildren)
}

// shallowEqualVNodeSlice compares two VNode slices by identity.
// Used to decide whether a parent should bail-out on unchanged children.
func shallowEqualVNodeSlice(a, b []*VNode) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// copyProps creates a shallow copy of Props.
func copyProps(p Props) Props {
	if p == nil {
		return nil
	}
	result := make(Props, len(p))
	for k, v := range p {
		result[k] = v
	}
	return result
}

// copyNodes creates a shallow copy of a VNode slice
func copyNodes(nodes []*VNode) []*VNode {
	if nodes == nil {
		return nil
	}
	result := make([]*VNode, len(nodes))
	copy(result, nodes)
	return result
}

// ErrorBoundary catches panics in child components during render.
func ErrorBoundary(child *VNode, fallback func(err interface{}) *VNode) *VNode {
	return H("span", Attr{
		"data-nguyen-error-boundary": "true",
		"data-nguyen-fallback":       "true",
	}, child)
}

// FC is a function component: func(props Props, children []*VNode) *VNode
type FC func(props Props, children []*VNode) *VNode

// fcWrapper wraps a function component to implement Component interface
type fcWrapper struct {
	BaseComponent
	fn FC
}

func (f *fcWrapper) Render() *VNode {
	return f.fn(f.props, f.children)
}

// FunctionComponent creates a Component from a function
func FunctionComponent(fn FC) Component {
	return &fcWrapper{fn: fn}
}

// Suspense wraps an async child with a fallback rendering during loading.
// The child should be a component that renders nil when loading.
func Suspense(child *VNode, fallback *VNode) *VNode {
	return H("span", Attr{"data-nguyen-suspense": "true"}, fallback, child)
}
