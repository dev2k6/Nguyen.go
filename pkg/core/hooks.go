//go:build js && wasm

package core

import (
	"reflect"
	"syscall/js"
)

// hooksStore holds all hooks for the current component being rendered
type hooksStore struct {
	hooks []*hook
	index int // current hook position during render
}

type hook struct {
	kind    int // 0=state, 1=effect, 2=context, 3=memo, 4=ref, 5=callback, 6=layoutEffect
	value   interface{}
	deps    []interface{} // dependencies for useEffect/useMemo
	cleanup func()        // useEffect cleanup function
	mounted bool
}

const (
	hookState        = 0
	hookEffect       = 1
	hookContext      = 2
	hookMemo         = 3
	hookRef          = 4
	hookCallback     = 5
	hookLayoutEffect = 6
)

// currentHooks tracks the active component's hooks during render.
// Set before calling component.Render() and reset after.
var currentHooks *hooksStore

// currentInstance is the component instance owning the currently rendering
// hooks context. Hook setters (setState, etc.) mark this instance dirty so
// the reconciler knows to re-render it instead of bailing out.
var currentInstance *componentInstance

// UseState is the reactive state hook — equivalent to React's useState.
// Returns the current value and a setter function.
// Calling the setter triggers a batched re-render of the component.
//
// Usage:
//
//	count, setCount := core.UseState(0)
//	setCount(count + 1) // triggers batched re-render
func UseState(initialValue interface{}) (interface{}, func(interface{})) {
	h := getOrCreateHook()
	if h.kind == 0 && !h.mounted {
		h.kind = hookState
		h.value = initialValue
		h.mounted = true
	}

	// Capture the instance owning this hook so setters can mark it dirty
	owner := currentInstance

	// The setter closure captures the hook reference
	setter := func(newVal interface{}) {
		h.value = newVal
		if owner != nil {
			owner.dirty = true
		}
		// Trigger batched re-render
		scheduleUpdate()
	}

	return h.value, setter
}

// UseEffect runs a side effect after render, similar to React's useEffect.
// Effects run after paint (async), unlike useLayoutEffect which runs before paint.
// If deps is provided, the effect only re-runs when dependencies change.
// Returns a cleanup function (optional — return nil from fn if no cleanup).
//
// Usage:
//
//	core.UseEffect(func() interface{} {
//	    document.Set("title", title)
//	    return nil
//	}, []interface{}{title})
func UseEffect(fn func() interface{}, deps []interface{}) {
	h := getOrCreateHook()
	scheduleEffect(h, fn, deps, false)
}

// UseLayoutEffect runs a side effect synchronously after all DOM mutations.
// Use this to read layout from the DOM and synchronously re-render.
// If deps is provided, the effect only re-runs when dependencies change.
//
// Usage:
//
//	core.UseLayoutEffect(func() interface{} {
//	    width := element.Get("clientWidth").Int()
//	    return nil
//	}, []interface{}{})
func UseLayoutEffect(fn func() interface{}, deps []interface{}) {
	h := getOrCreateHook()
	scheduleEffect(h, fn, deps, true)
}

// scheduleEffect schedules an effect hook with dependency checking
func scheduleEffect(h *hook, fn func() interface{}, deps []interface{}, isLayout bool) {
	// Check if deps changed (React-style areHookInputsEqual)
	depsChanged := !h.mounted || !areHookInputsEqual(deps, h.deps)

	if depsChanged {
		// Run cleanup from previous effect
		if h.cleanup != nil {
			h.cleanup()
			h.cleanup = nil
		}

		// Queue effect to run after render
		if isLayout {
			h.value = fn // store fn for layout effect flush
		} else {
			// Run async after paint
			js.Global().Call("setTimeout", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
				result := fn()
				if result != nil {
					if cleanupFn, ok := result.(func()); ok {
						h.cleanup = cleanupFn
					}
				}
				return nil
			}), 0)
		}

		h.deps = deps
	}

	if isLayout {
		h.kind = hookLayoutEffect
	} else {
		h.kind = hookEffect
	}
	h.mounted = true
}

// areHookInputsEqual mirrors React's areHookInputsEqual.
// Uses Object.is-style comparison: NaN === NaN, +0 !== -0, reflect.DeepEqual
// as fallback for complex types. Returns false if length differs or prev is nil.
func areHookInputsEqual(next, prev []interface{}) bool {
	if prev == nil {
		return false
	}
	if len(next) != len(prev) {
		return false
	}
	for i := range prev {
		if !objectIs(next[i], prev[i]) {
			return false
		}
	}
	return true
}

// objectIs implements JS Object.is semantics for Go interface{} values.
// Handles nil, comparable scalars, and NaN identity. Falls back to
// reflect.DeepEqual for slices/maps/structs.
func objectIs(a, b interface{}) bool {
	if a == nil || b == nil {
		return a == b
	}
	// Fast path: comparable types (bool, int, string, ptr, etc.)
	if reflect.TypeOf(a).Comparable() && reflect.TypeOf(b).Comparable() {
		defer func() { _ = recover() }()
		return a == b
	}
	// Fallback for incomparable types (maps, slices, funcs)
	return reflect.DeepEqual(a, b)
}

// UseMemo memoizes a computed value, only recomputing when deps change.
// Equivalent to React's useMemo.
//
// Usage:
//
//	expensiveValue := core.UseMemo(func() interface{} {
//	    return computeExpensive(items)
//	}, []interface{}{items})
func UseMemo(fn func() interface{}, deps []interface{}) interface{} {
	h := getOrCreateHook()

	if !h.mounted || !areHookInputsEqual(deps, h.deps) {
		h.value = fn()
		h.deps = deps
	}

	h.kind = hookMemo
	h.mounted = true
	return h.value
}

// UseCallback returns a memoized callback. Only changes if deps change.
// Equivalent to React's useCallback.
//
// Usage:
//
//	handleClick := core.UseCallback(func() {
//	    setCount(count + 1)
//	}, []interface{}{count})
func UseCallback(fn func(), deps []interface{}) func() {
	h := getOrCreateHook()

	if !h.mounted || !areHookInputsEqual(deps, h.deps) {
		h.value = fn
		h.deps = deps
	}

	h.kind = hookCallback
	h.mounted = true

	if f, ok := h.value.(func()); ok {
		return f
	}
	return fn
}

// UseContext accesses a context value from a parent provider.
// Similar to React's useContext.
//
// Usage:
//
//	theme := core.UseContext(ThemeContext)
func UseContext(ctx *Context) interface{} {
	_ = getOrCreateHook() // reserve slot
	return ctx.Value()
}

// UseRef creates a mutable reference that persists across renders.
// Similar to React's useRef.
//
// Usage:
//
//	inputRef := core.UseRef(nil)
//	inputRef.Value = domElement
func UseRef(initial interface{}) *Ref {
	h := getOrCreateHook()
	if !h.mounted {
		h.kind = hookRef
		h.value = &Ref{Value: initial}
		h.mounted = true
	}
	return h.value.(*Ref)
}

// Ref is a mutable reference container
type Ref struct {
	Value interface{}
}

// GetDOM returns the ref value as a DOM element (js.Value)
func (r *Ref) GetDOM() js.Value {
	if r == nil || r.Value == nil {
		return js.Null()
	}
	if v, ok := r.Value.(js.Value); ok {
		return v
	}
	return js.Null()
}

// Context provides a way to pass data through the component tree
// without passing props manually at every level.
type Context struct {
	value interface{}
}

// NewContext creates a new Context with a default value
func NewContext(defaultValue interface{}) *Context {
	return &Context{value: defaultValue}
}

// Value returns the current context value
func (c *Context) Value() interface{} {
	return c.value
}

// SetValue updates the context value (called by Provider)
func (c *Context) SetValue(v interface{}) {
	c.value = v
}

// Provider wraps children with a context value
type Provider struct {
	BaseComponent
	ctx   *Context
	value interface{}
}

func (p *Provider) Render() *VNode {
	p.ctx.SetValue(p.value)
	if len(p.children) > 0 {
		return p.children[0]
	}
	return nil
}

// CreateProvider creates a Context Provider component
func CreateProvider(ctx *Context, value interface{}, children ...*VNode) *VNode {
	prov := &Provider{ctx: ctx, value: value}
	prov.SetChildren(children)
	return ComponentVNode(prov, nil, children...)
}

// getOrCreateHook returns the current hook, creating it if necessary
func getOrCreateHook() *hook {
	if currentHooks == nil {
		currentHooks = &hooksStore{hooks: make([]*hook, 0), index: -1}
	}
	currentHooks.index++
	if currentHooks.index >= len(currentHooks.hooks) {
		currentHooks.hooks = append(currentHooks.hooks, &hook{})
	}
	return currentHooks.hooks[currentHooks.index]
}

// ===================== SCHEDULER (React Fiber-style) =====================

// Lane is a bitmask encoding update priority (React-style).
// Lower bit = higher priority. Use getHighestPriorityLane to peel the most
// urgent update when deciding what to flush first.
type Lane uint32

const (
	NoLane             Lane = 0b00000000000000000000000000000000
	SyncLane           Lane = 0b00000000000000000000000000000010 // user events (click, input)
	InputContinuousLane Lane = 0b00000000000000000000000000001000 // scroll, drag
	DefaultLane        Lane = 0b00000000000000000000000000100000 // setState default
	TransitionLane     Lane = 0b00000000000000000000001000000000 // useTransition
	IdleLane           Lane = 0b00100000000000000000000000000000 // background work
)

// getHighestPriorityLane isolates the lowest set bit (highest priority).
// Two's complement trick from ReactFiberLane.js:756.
func getHighestPriorityLane(lanes Lane) Lane {
	return lanes & -lanes
}

// pendingUpdate represents a scheduled update with priority.
type pendingUpdate struct {
	fn   func()
	lane Lane
}

// UpdateQueue holds pending component updates to be processed in a single batch.
// This implements React's automatic batching: multiple setState calls in the
// same event handler are merged into one re-render.
// WASM is single-threaded — no mutex needed.
type UpdateQueue struct {
	pending   []pendingUpdate
	scheduled bool
	laneMask  Lane // OR of all pending lanes
}

var updateQueue = &UpdateQueue{}

// scheduleUpdate schedules a component re-render at DefaultLane priority.
// Multiple updates in the same tick are batched together.
func scheduleUpdate() {
	scheduleUpdateLane(DefaultLane)
}

// scheduleUpdateLane schedules a re-render at a specific priority lane.
// SyncLane flushes immediately (microtask); lower priorities batch via setTimeout.
func scheduleUpdateLane(lane Lane) {
	updateQueue.pending = append(updateQueue.pending, pendingUpdate{
		fn:   func() { NotifyStateChange() },
		lane: lane,
	})
	updateQueue.laneMask |= lane

	if updateQueue.scheduled {
		return
	}
	updateQueue.scheduled = true

	// SyncLane → microtask (queueMicrotask), else setTimeout 0
	if lane == SyncLane {
		js.Global().Call("queueMicrotask", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			flushUpdateQueue()
			return nil
		}))
		return
	}

	js.Global().Call("setTimeout", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		flushUpdateQueue()
		return nil
	}), 0)
}

// flushUpdateQueue processes all pending updates in a single batch.
// Sync-lane updates run first so user interactions stay snappy.
func flushUpdateQueue() {
	if len(updateQueue.pending) == 0 {
		updateQueue.scheduled = false
		updateQueue.laneMask = NoLane
		return
	}

	// Snapshot current work (React-style: new updates scheduled during flush
	// go into a fresh queue and fire next tick)
	pending := updateQueue.pending
	updateQueue.pending = nil
	updateQueue.scheduled = false
	updateQueue.laneMask = NoLane

	// Process highest-priority lane first (lowest bit)
	highest := NoLane
	for _, u := range pending {
		if highest == NoLane || u.lane < highest {
			highest = u.lane
		}
	}

	// Run highest priority first, then rest in order
	for _, u := range pending {
		if u.lane == highest {
			u.fn()
		}
	}
	for _, u := range pending {
		if u.lane != highest {
			u.fn()
		}
	}
}

// FlushSync forces synchronous execution of pending updates.
// Use sparingly — defeats batching. Equivalent to React's flushSync.
func FlushSync(fn func()) {
	fn()
	flushUpdateQueue()
}

// --- State change notification system (connects hooks to reconciler) ---

var stateChangeListeners []func()

// OnStateChange registers a callback for state changes
func OnStateChange(fn func()) {
	stateChangeListeners = append(stateChangeListeners, fn)
}

// NotifyStateChange notifies listeners that state has changed
func NotifyStateChange() {
	listeners := make([]func(), len(stateChangeListeners))
	copy(listeners, stateChangeListeners)
	for _, fn := range listeners {
		fn()
	}
}

// --- App root rendering ---

// AppRoot holds the root VNode tree and handles re-rendering
type AppRoot struct {
	reconciler  *Reconciler
	rootElement js.Value
	oldTree     *VNode
	renderFn    func() *VNode
}

// CreateApp creates a new application root
func CreateApp(rootID string, renderFn func() *VNode) *AppRoot {
	root := GetElementById(rootID)
	return &AppRoot{
		reconciler:  NewReconciler(),
		rootElement: root,
		renderFn:    renderFn,
	}
}

// Mount performs the initial render
func (a *AppRoot) Mount() {
	newTree := a.renderFn()
	a.reconciler.Render(a.rootElement, a.oldTree, newTree)
	a.oldTree = newTree

	// Install event delegation
	SetupEventDelegation(a.rootElement)

	// Register for state changes
	OnStateChange(func() {
		a.Rerender()
	})
}

// Rerender re-renders the app (called on state changes)
func (a *AppRoot) Rerender() {
	newTree := a.renderFn()
	a.reconciler.Render(a.rootElement, a.oldTree, newTree)
	a.oldTree = newTree
}

// Event delegation — maps event types to handlers at the app root
var delegatedEvents = map[string]bool{
	"click":      true,
	"input":      true,
	"change":     true,
	"submit":     true,
	"keydown":    true,
	"keyup":      true,
	"focus":      true,
	"blur":       true,
	"mouseenter": true,
	"mouseleave": true,
	"scroll":     true,
}

// SetupEventDelegation installs event listeners on the root element
// to catch all events via bubbling (event delegation).
// Component event handlers (onClick, etc.) are found by traversing
// the DOM tree from the event target upward.
func SetupEventDelegation(root js.Value) {
	for eventType := range delegatedEvents {
		eventType := eventType // capture
		root.Call("addEventListener", eventType, js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			event := args[0]
			target := event.Get("target")

			// Walk up the DOM tree looking for a data-nguyen-handler attribute
			current := target
			for !current.IsNull() && !current.IsUndefined() {
				handlerName := current.Call("getAttribute", "data-nguyen-"+eventType)
				if !handlerName.IsNull() && handlerName.Type() == js.TypeString && handlerName.String() != "" {
					// Call the registered handler
					name := handlerName.String()
					if handler, ok := eventHandlers[name]; ok {
						handler(event)
						return nil
					}
				}
				current = current.Get("parentElement")
			}
			return nil
		}))
	}
}

// eventHandlers stores registered event handlers by name
var eventHandlers = make(map[string]func(js.Value))

// RegisterEventHandler registers a named event handler for delegation
func RegisterEventHandler(name string, handler func(js.Value)) {
	eventHandlers[name] = handler
}

// BindEventAttr sets a data-nguyen-{event} attribute on a DOM element for delegation.
// Instead of addEventListener per element, we use one listener at root.
func BindEventAttr(el js.Value, eventType, handlerName string) {
	el.Call("setAttribute", "data-nguyen-"+eventType, handlerName)
}
