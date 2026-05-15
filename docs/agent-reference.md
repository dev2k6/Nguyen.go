# Nguyen.go — AI Agent Reference

This document is designed for AI coding assistants (GitHub Copilot, Claude, Cursor, Cody, etc.) to understand the Nguyen.go framework syntax and conventions.

## What is Nguyen.go?

Nguyen.go is a full-stack Go web framework. It uses:
- **Go** for backend (Fiber HTTP server)
- **TinyGo → WebAssembly** for frontend (React-style virtual DOM)
- **`.gox` files** as single-file components (Go frontmatter + HTML template)

## `.gox` File Format

A `.gox` file has two sections separated by `---`:

```
---
[Go frontmatter: imports, state, handlers, metadata]
---

[HTML template: reactive markup with {interpolation}]
```

### Rules

1. Everything between the first `---` and second `---` is Go code
2. Everything after the second `---` is HTML template
3. If no `---` exists, the entire file is treated as HTML template
4. The Go section is NOT compiled by `go build` directly — it's parsed and transpiled by the Nguyen.go compiler

## Frontmatter Syntax

The frontmatter looks like Go but has framework-specific extensions:

### Standard Go

```go
import "nguyen.go/pkg/core"
import "fmt"

count, setCount := core.UseGlobalState("count", 0)

func increment() {
    setCount(count.(int) + 1)
}
```

### Framework Extensions (NOT standard Go)

```go
// Metadata declaration (parsed as key-value, not valid Go)
export const metadata = {
    title: "Page Title",
    description: "Page description",
    og:image: "/og.png",
}

// GEO structured data
export const geo = {
    pageType: "Article",
    datePublished: "2026-01-01T00:00:00Z",
    breadcrumbs: [
        {name: "Home", url: "/"},
    ],
    faqs: [
        {question: "Q?", answer: "A."},
    ],
}

// ISR revalidation time (seconds)
export const revalidate = 60

// Layout reference
layout = "app/layout"

// Route guard
guard = "auth"
```

These `export const` blocks use JavaScript-like object syntax intentionally — they are parsed by the Nguyen.go compiler, not the Go compiler.

## Template Syntax

### Variable Interpolation

```html
<h1>Hello, {name}!</h1>
<p>Count: {count}</p>
```

Renders as reactive `<span data-nguyen-text="key">value</span>` in SSR.

### Event Bindings

```html
<button @click="handlerName()">Click</button>
<input @input="onInput()" />
<form @submit="onSubmit()">...</form>
```

Pattern: `@eventName="functionName()"`

Supported: `@click`, `@dblclick`, `@input`, `@change`, `@submit`, `@keydown`, `@keyup`, `@focus`, `@blur`, `@mouseenter`, `@mouseleave`, `@mouseover`, `@mouseout`

### Control Flow

```html
<ng-if condition="variableName">
    <p>Shown when truthy</p>
</ng-if>
<ng-else-if condition="otherVar">
    <p>Alternative</p>
</ng-else-if>
<ng-else>
    <p>Fallback</p>
</ng-else>

<ng-for items="listVar" item="itemVar" index="i">
    <li>{itemVar}</li>
</ng-for>
```

### Links (SPA Navigation)

```html
<nguyen-link href="/about">About</nguyen-link>
```

Renders as `<a>` in SSR, intercepts clicks for client-side navigation.

### Slots (in layouts)

```html
<!-- Default slot -->
<nguyen-slot />

<!-- Named slot -->
<nguyen-slot name="sidebar" />

<!-- Head placeholder (for meta injection) -->
<nguyen-head />
```

### Scoped CSS

```html
<style scoped>
.card { padding: 2rem; }
</style>
```

## State Hooks (pkg/core)

```go
// Local state
value, setValue := core.UseState(initialValue)

// Global state (shared across components)
value, setValue := core.UseGlobalState("key", initialValue)

// Read global state
val := core.GetGlobalState("key")

// Set global state (outside component)
core.SetGlobalState("key", newValue)

// Side effects
core.UseEffect(func() interface{} {
    // effect code
    return cleanupFn // or nil
}, []interface{}{deps...})

// Layout effect (sync, before paint)
core.UseLayoutEffect(func() interface{} { ... }, deps)

// Memoization
result := core.UseMemo(func() interface{} { return expensive() }, deps)

// Memoized callback
fn := core.UseCallback(func() { ... }, deps)

// Ref (persists across renders, no re-render on change)
ref := core.UseRef(nil)

// Context
ctx := core.NewContext(defaultValue)
core.CreateProvider(ctx, value, children...)
val := core.UseContext(ctx)

// Search params
params := core.UseSearchParams() // map[string]string
```

## Component Interface

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

// Embed BaseComponent for defaults
type MyComp struct {
    core.BaseComponent
}
func (m *MyComp) Render() *core.VNode { ... }
```

## VNode Creation

```go
// Element
core.H("div", core.Attr{"class": "box"}, children...)

// Text
core.Text("hello")

// Component
core.ComponentVNode(comp, props, children...)

// Fragment
core.Fragment(node1, node2, node3)
```

## Islands (Partial Hydration)

```go
core.Island(core.IslandConfig{
    Directive: core.HydrateVisible, // Load|Idle|Visible|Media|Only|None
    Name:      "Counter",
    Props:     core.Props{"initial": 0},
}, childVNode)
```

## Routing Conventions

| File | Route |
|------|-------|
| `pages/index.gox` | `/` |
| `pages/about.gox` | `/about` |
| `pages/blog/[slug].gox` | `/blog/:slug` |
| `pages/docs/[...path].gox` | `/docs/*` |
| `pages/[[...opt]].gox` | optional catch-all |
| `pages/404.gox` | fallback |
| `pages/layout.gox` | layout wrapper |

## Configuration (nguyen.config.yml)

Key settings:

```yaml
render:
  mode: ssr|csr|isr
  stream: true|false
pwa:
  enabled: true|false
geo:
  enabled: true|false
```

## CLI Commands

```bash
nguyen create <name>           # scaffold project
nguyen dev                     # dev server + HMR
nguyen build                   # production build
nguyen start                   # serve build output
nguyen export                  # static HTML export
nguyen check                   # syntax validation
nguyen doctor                  # diagnose toolchain + project layout (v1.1.0+)
nguyen vet                     # static analysis on .gox files (v1.1.0+)
nguyen generate routes         # typed route accessors (v1.1.0+)
nguyen dist                    # reproducible cross-compiled binaries (v1.1.0+)
```

### `nguyen vet` rules (v1.1.0+)

When generating .gox code, AI assistants should write code that passes
the default vet rules. These mirror common Go idioms; ignore them only
when you have a concrete reason.

| Rule | Severity | Fix |
|---|---|---|
| `context-first-param` | warning | `Load`, `Action`, `Stream`, `Handle`, `GET`/`POST`/... must take `context.Context` as first parameter |
| `no-time-sleep` | warning | Replace `time.Sleep(d)` with `select { case <-time.After(d): case <-ctx.Done(): }` |
| `goroutine-cancellation` | warning | Every spawned goroutine must reference `ctx` so it exits when the request ends |
| `no-hardcoded-secrets` | error | Load API keys / passwords / tokens from `os.Getenv`, never inline |
| `no-fmt-print` | warning | Use `observe.Logger(ctx).Info(...)` instead of `fmt.Println` / `fmt.Printf` in handlers |
| `event-handler-exists` | error | Every `@click="foo()"` requires `func foo()` declared in frontmatter |

## Common Patterns

### Counter Page

```nguyen
---
import "nguyen.go/pkg/core"

count, setCount := core.UseGlobalState("count", 0)

func increment() {
    setCount(count.(int) + 1)
}

func decrement() {
    setCount(count.(int) - 1)
}

export const metadata = {
    title: "Counter",
}
---

<section class="text-center p-8">
    <h1>Count: {count}</h1>
    <button @click="increment()">+</button>
    <button @click="decrement()">-</button>
</section>
```

### Layout

```nguyen
---
export const metadata = {
    title: "My App",
}
---

<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <link rel="stylesheet" href="/styles/output.css">
    <nguyen-head />
</head>
<body>
    <nav>
        <nguyen-link href="/">Home</nguyen-link>
        <nguyen-link href="/about">About</nguyen-link>
    </nav>
    <main>
        <nguyen-slot />
    </main>
</body>
</html>
```

### API Route (pure Go)

```go
// api/routes.go
package api

import "github.com/gofiber/fiber/v2"

func Register(app fiber.Router) {
    api := app.Group("/api")
    api.Get("/health", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{"status": "ok"})
    })
}
```

## Key Differences from Standard Go

1. `export const metadata = { ... }` — NOT Go syntax, parsed by framework
2. `export const geo = { ... }` — NOT Go syntax, parsed by framework
3. `export const revalidate = 60` — NOT Go syntax, parsed by framework
4. `layout = "path"` — layout reference, parsed by framework
5. `guard = "name"` — route guard, parsed by framework
6. `{variable}` in HTML — template interpolation, NOT Go string formatting
7. `@event="handler()"` — event binding, NOT Go syntax
8. `<ng-if>`, `<ng-for>` — control flow directives, NOT standard HTML

## Build Tags

- `pkg/core` uses `//go:build js && wasm` — only compiles for WASM target
- Backend code (`internal/`, `cmd/`) compiles normally for any OS
- The `.gox` compiler transpiles frontmatter into valid Go for TinyGo

## Module Path

```
nguyen.go                       # module name in go.mod
nguyen.go/pkg/core              # WASM runtime (hooks, VNode, reconciler)
nguyen.go/pkg/nguyen            # Server library (App, Options)
nguyen.go/pkg/concurrent        # Pool, Map, Parallel, Race, Deadline, Lifecycle (v1.1.0+)
nguyen.go/pkg/ngctx             # Standard context keys (v1.1.0+)
nguyen.go/pkg/observe           # slog logger + Span timing (v1.1.0+)
nguyen.go/internal/parser       # .gox file parser
nguyen.go/internal/router       # file-system router
nguyen.go/internal/render       # SSR engine
nguyen.go/internal/compiler     # TinyGo WASM compiler
nguyen.go/internal/server       # Fiber server + HMR + ContextMiddleware
nguyen.go/internal/config       # YAML config loader
nguyen.go/internal/cache        # ISR cache
nguyen.go/internal/geo          # SEO/GEO engine
nguyen.go/internal/pwa          # PWA manifest/SW generator
nguyen.go/internal/optimizer    # Image optimization
nguyen.go/internal/vet          # .gox static analysis (v1.1.0+)
nguyen.go/internal/routegen     # Typed route codegen (v1.1.0+)
```

## Server Handler Conventions (v1.1.0+)

Loaders, actions and streams take `context.Context` as the first
parameter. The framework's `ContextMiddleware` populates the context
with trace ID, request ID, locale and authenticated user before any
handler runs.

```go
import (
    "context"
    "github.com/dev2k6/Nguyen.go/pkg/ngctx"
    "github.com/dev2k6/Nguyen.go/pkg/observe"
    "github.com/dev2k6/Nguyen.go/pkg/concurrent"
)

// Loader pattern — fetch in parallel, log structured events
func Load(ctx context.Context, p Params) (PageData, error) {
    log := observe.Logger(ctx)
    log.Info("page.load", "id", p["id"])

    var user User
    var posts []Post
    err := concurrent.Parallel(ctx,
        func(c context.Context) error { user, _ = loadUser(c, p["id"]); return nil },
        func(c context.Context) error { posts, _ = loadPosts(c, p["id"]); return nil },
    )
    if err != nil {
        return PageData{}, err
    }
    return PageData{User: user, Posts: posts}, nil
}

// Action pattern — read user from ctx, never trust the form alone
func Action(ctx context.Context, form FormData) error {
    u := ngctx.User(ctx)
    if u == nil || !u.HasRole("editor") {
        return errForbidden
    }
    return savePost(ctx, u.ID, form)
}
```

### Anti-patterns (vet will warn)

```go
// BAD — ignores ctx, blocks goroutine
time.Sleep(time.Second)

// GOOD
select {
case <-time.After(time.Second):
case <-ctx.Done():
    return ctx.Err()
}

// BAD — goroutine outlives request
go func() {
    doWork()
}()

// GOOD
go func() {
    select {
    case <-ctx.Done():
        return
    default:
        doWork(ctx)
    }
}()

// BAD — unstructured, bypasses request scope
fmt.Println("user logged in:", id)

// GOOD
observe.Logger(ctx).Info("auth.login", "user_id", id)

// BAD — secret in source
const apiKey = "sk_live_abcdef1234567890"

// GOOD
apiKey := os.Getenv("STRIPE_SECRET_KEY")
```

## Type-Safe Routes (v1.1.0+)

After running `nguyen generate routes`, link to pages through the
generated package instead of building strings by hand.

```go
import "myapp/routes"

// Static
href := routes.About()                    // "/about"

// Dynamic
href := routes.BlogBySlug(post.Slug)      // "/blog/hello-world"
```

A typo (`routes.BlogBySlugg`) becomes a compile error rather than a
404 at runtime. Run `nguyen generate routes` whenever you add or
rename a `.gox` file.
