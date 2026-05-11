# Hooks & State

Nguyen.go provides React-style hooks for state management and side effects. These hooks run in the WASM runtime (client-side) and power the reactive UI.

## UseState

Local component state. Returns the current value and a setter function.

```go
count, setCount := core.UseState(0)
setCount(count.(int) + 1)  // triggers re-render
```

Multiple setState calls in the same event handler are batched into a single re-render (automatic batching).

## UseGlobalState

App-level shared state accessible from any component.

```go
theme, setTheme := core.UseGlobalState("theme", "dark")
setTheme("light")  // all components reading "theme" re-render
```

### Reading Global State

```go
value := core.GetGlobalState("theme")
```

### Setting Global State (outside components)

```go
core.SetGlobalState("theme", "light")
```

## UseEffect

Runs a side effect after render (async, after paint). Equivalent to React's `useEffect`.

```go
core.UseEffect(func() interface{} {
    // Side effect: update document title
    js.Global().Get("document").Set("title", title)

    // Return cleanup function (or nil)
    return func() {
        // cleanup runs before next effect or unmount
    }
}, []interface{}{title})  // dependency array
```

### Rules

- Empty deps `[]interface{}{}` — runs once after mount
- With deps — runs when any dependency changes
- No deps (nil) — runs after every render

## UseLayoutEffect

Runs synchronously after DOM mutations, before the browser paints. Use for DOM measurements.

```go
core.UseLayoutEffect(func() interface{} {
    width := element.Get("clientWidth").Int()
    setWidth(width)
    return nil
}, []interface{}{})
```

## UseMemo

Memoizes an expensive computation. Only recomputes when dependencies change.

```go
expensiveResult := core.UseMemo(func() interface{} {
    return computeExpensiveValue(items)
}, []interface{}{items})
```

## UseCallback

Returns a memoized callback function. Only changes when dependencies change.

```go
handleClick := core.UseCallback(func() {
    setCount(count.(int) + 1)
}, []interface{}{count})
```

## UseRef

Creates a mutable reference that persists across renders. Does not trigger re-renders when changed.

```go
inputRef := core.UseRef(nil)

// Later: access the DOM element
element := inputRef.GetDOM()
element.Call("focus")
```

## UseContext

Access a context value from a parent Provider. Avoids prop drilling.

```go
// Create context
var ThemeContext = core.NewContext("light")

// Provide value
core.CreateProvider(ThemeContext, "dark", children...)

// Consume value
theme := core.UseContext(ThemeContext)
```

## UseSearchParams

Access URL query parameters reactively. Re-renders when navigation changes.

```go
params := core.UseSearchParams()
page := params["page"]    // ?page=2
sort := params["sort"]    // ?sort=name
```

## Scheduler & Priority Lanes

Nguyen.go uses a React Fiber-style lane-based scheduler for update prioritization:

| Lane | Priority | Use Case |
|------|----------|----------|
| `SyncLane` | Highest | User events (click, input) |
| `InputContinuousLane` | High | Scroll, drag |
| `DefaultLane` | Normal | setState default |
| `TransitionLane` | Low | useTransition |
| `IdleLane` | Lowest | Background work |

Higher-priority updates interrupt lower-priority ones. Multiple updates in the same tick are batched.

### FlushSync

Force synchronous execution (defeats batching — use sparingly):

```go
core.FlushSync(func() {
    setCount(newValue)
})
// DOM is updated here
```

## SSR Hydration State

In SSR mode, state declared in frontmatter is rendered server-side and hydrated on the client:

```nguyen
---
count, setCount := core.UseGlobalState("count", 0)
---

<p>Count: {count}</p>
```

Server output:
```html
<p>Count: <span data-nguyen-text="count">0</span></p>
```

The hydration script reads `data-nguyen-text` attributes to initialize client state, enabling seamless SSR → CSR transition.

## Event Handlers in SSR

Event handlers defined in frontmatter are transpiled to client-side JavaScript:

```nguyen
---
func increment() {
    core.SetGlobalState("count", core.GetGlobalState("count").(int) + 1)
}
---

<button @click="increment()">+</button>
```

The SSR hydration script provides:
- `window.__nguyenState` — reactive state store
- `window.__nguyenSetState(key, value)` — state setter
- `window.__nguyenHandlers` — event handler registry
- Automatic event delegation via `data-nguyen-{event}` attributes
