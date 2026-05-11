# Components

Nguyen.go provides a component system built on virtual DOM nodes (VNodes) with a React-style reconciler.

## VNode (Virtual DOM)

The core data structure representing UI elements:

```go
type VNode struct {
    Type      VNodeType          // Element, Text, Component, Empty
    Tag       string             // HTML tag name
    Props     map[string]string  // HTML attributes
    Style     map[string]string  // Inline styles
    Text      string             // Text content
    Children  []*VNode           // Child nodes
    Key       string             // Reconciliation key
    Component Component          // Component constructor
    CompProps Props              // Component props
}
```

### Creating VNodes

```go
// Element
node := core.H("div", core.Attr{"class": "container"},
    core.H("h1", nil, "Hello"),
    core.H("p", nil, "World"),
)

// Text
text := core.Text("Hello, world!")

// Fragment (no wrapper element)
nodes := core.Fragment(
    core.H("li", nil, "Item 1"),
    core.H("li", nil, "Item 2"),
)
```

### Attr Helper

```go
props := core.Attr{
    "class":       "btn btn-primary",
    "id":          "submit-btn",
    "aria-label":  "Submit form",
    "data-action": "submit",
}

// Merge multiple attr maps
merged := core.MergeProps(baseAttrs, overrideAttrs)
```

## Component Interface

All components implement:

```go
type Component interface {
    Render() *VNode
    SetProps(props Props)
    Props() Props
    SetChildren(children []*VNode)
    Children() []*VNode
    Key() string
    SetKey(key string)
}
```

### BaseComponent

Embed `BaseComponent` for default implementations:

```go
type MyComponent struct {
    core.BaseComponent
}

func (m *MyComponent) Render() *VNode {
    name := core.PropValue[string](m.Props(), "name")
    return core.H("div", nil,
        core.H("h1", nil, "Hello, "+name),
    )
}
```

### Function Components

Simpler syntax for stateless components:

```go
var Greeting = core.FunctionComponent(func(props core.Props, children []*core.VNode) *core.VNode {
    name := core.PropValue[string](props, "name")
    return core.H("p", nil, "Hello, "+name+"!")
})
```

### Using Components

```go
node := core.ComponentVNode(
    &MyComponent{},
    core.Props{"name": "World"},
    // children...
)
```

## Props

Props use `map[string]interface{}` for flexibility:

```go
type Props map[string]interface{}

// Type-safe access
name := core.PropValue[string](props, "name")
count := core.PropValue[int](props, "count")
items := core.PropValue[[]string](props, "items")
```

## Memo (React.memo)

Wrap a component to skip re-renders when props haven't changed:

```go
memoized := core.Memo(&ExpensiveComponent{}, nil)
// Uses shallow comparison by default

// Custom comparison
memoized := core.Memo(&ExpensiveComponent{}, func(old, new core.Props) bool {
    return old["id"] == new["id"]  // only re-render if id changes
})
```

## Suspense

Show a fallback while async content loads:

```go
node := core.Suspense(
    asyncChild,                              // component that may render nil while loading
    core.H("div", nil, "Loading..."),        // fallback
)
```

## ErrorBoundary

Catch panics in child components:

```go
node := core.ErrorBoundary(
    riskyChild,
    func(err interface{}) *core.VNode {
        return core.H("div", core.Attr{"class": "error"},
            core.Text("Something went wrong: "+fmt.Sprint(err)),
        )
    },
)
```

## Islands Architecture

Islands enable partial hydration — only interactive components load WASM, the rest stays static HTML.

### Hydration Directives

| Directive | Behavior |
|-----------|----------|
| `HydrateNone` | Fully static, no client JS |
| `HydrateLoad` | Mount immediately on page load |
| `HydrateIdle` | Mount when `requestIdleCallback` fires |
| `HydrateVisible` | Mount when element enters viewport |
| `HydrateMedia` | Mount when media query matches |
| `HydrateOnly` | Skip SSR, render client-only |

### Creating an Island

```go
node := core.Island(core.IslandConfig{
    Directive: core.HydrateVisible,
    Name:      "Counter",
    Props:     core.Props{"initial": 0},
}, counterVNode)
```

### Registering Island Components

```go
core.RegisterIsland("Counter", func(props core.Props) core.Component {
    return &CounterComponent{}
})
```

### Hydrating Islands

Call once after the page loads:

```go
core.HydrateIslands()
```

This scans the DOM for `data-nguyen-island` markers and schedules each island based on its directive.

### SSR Output

Islands render as custom elements with metadata:

```html
<nguyen-island
    data-nguyen-island="visible"
    data-nguyen-island-c="Counter"
    data-nguyen-island-p='{"initial":0}'>
    <!-- SSR placeholder content -->
</nguyen-island>
```

## Reconciler

The reconciler diffs old and new VNode trees and applies minimal DOM mutations:

1. **Same type at same position** → patch props/text, recurse children
2. **Different type** → replace entire subtree
3. **Keyed children** → reorder instead of destroy+create

### Key Prop

Use keys for efficient list rendering:

```go
for _, item := range items {
    node := core.H("li", core.Attr{"key": item.ID}, item.Name)
    children = append(children, node)
}
```

Keys help the reconciler identify which items moved, were added, or removed.

## Event Delegation

Events are handled via delegation at the app root (one listener per event type):

```go
// Register a handler
core.RegisterEventHandler("increment", func(event js.Value) {
    setCount(count.(int) + 1)
})

// Bind to element
core.BindEventAttr(element, "click", "increment")
```

In templates, use `@event` syntax:
```html
<button @click="increment()">+</button>
```

## App Root

Mount your application:

```go
func main() {
    app := core.CreateApp("app", func() *core.VNode {
        return core.H("div", nil,
            core.H("h1", nil, "My App"),
        )
    })
    app.Mount()

    // Keep WASM alive
    select {}
}
```

## Client-Side Router

SPA navigation without full page reloads:

```go
// Navigate programmatically
core.Navigate("/blog/new-post")

// Listen for navigation events
core.OnNavigate(func(path string) {
    // update UI based on new path
})

// Get current path
path := core.CurrentPath()

// Check active state
isActive := core.IsActivePath("/blog", false)
```

The router handles browser back/forward via `popstate` events automatically.
