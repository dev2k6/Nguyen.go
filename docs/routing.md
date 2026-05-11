# Routing

Nguyen.go uses file-system routing. Every `.nguyen` file in the `pages/` directory becomes a route automatically.

## Basic Routes

| File | Route |
|------|-------|
| `pages/index.nguyen` | `/` |
| `pages/about.nguyen` | `/about` |
| `pages/blog.nguyen` | `/blog` |
| `pages/contact.nguyen` | `/contact` |

## Nested Routes

Subdirectories create nested URL paths:

| File | Route |
|------|-------|
| `pages/blog/index.nguyen` | `/blog` |
| `pages/blog/first-post.nguyen` | `/blog/first-post` |
| `pages/docs/getting-started.nguyen` | `/docs/getting-started` |

## Dynamic Routes

Use brackets `[param]` for dynamic segments:

| File | Route | Example |
|------|-------|---------|
| `pages/blog/[slug].nguyen` | `/blog/:slug` | `/blog/hello-world` |
| `pages/users/[id].nguyen` | `/users/:id` | `/users/42` |
| `pages/[category]/[id].nguyen` | `/:category/:id` | `/tech/123` |

Access params in your page via the route context.

## Catch-All Routes

Use `[...param]` for catch-all segments:

| File | Route | Matches |
|------|-------|---------|
| `pages/docs/[...path].nguyen` | `/docs/*` | `/docs/a/b/c` |

## Optional Catch-All

Use `[[...param]]` for optional catch-all (matches with or without the segment):

| File | Route | Matches |
|------|-------|---------|
| `pages/shop/[[...slug]].nguyen` | `/shop`, `/shop/a/b` | Both |

## 404 Page

Create `pages/404.nguyen` for a custom not-found page. It has the lowest priority and catches all unmatched routes.

```nguyen
---
export const metadata = {
    title: "Page Not Found",
}
---

<section class="p-8 text-center">
    <h1 class="text-4xl">404</h1>
    <p>The page you're looking for doesn't exist.</p>
    <nguyen-link href="/" class="btn mt-4">Go Home</nguyen-link>
</section>
```

## Route Priority

Routes are evaluated in priority order (most specific first):

1. **Static routes** — `/about`, `/blog/first-post` (priority 0)
2. **Dynamic routes** — `/blog/:slug` (priority 10 per dynamic segment)
3. **Optional catch-all** — `/shop/[[...slug]]` (priority 50)
4. **Catch-all** — `/docs/[...path]` (priority 100)
5. **404** — `/*` (priority 1000)

## Layouts

### Root Layout

Create `pages/layout.nguyen` to wrap all pages:

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
    <header>
        <nav>
            <nguyen-link href="/">Home</nguyen-link>
            <nguyen-link href="/about">About</nguyen-link>
            <nguyen-link href="/blog">Blog</nguyen-link>
        </nav>
    </header>
    <main>
        <nguyen-slot />
    </main>
    <footer>
        <p>Built with Nguyen.go</p>
    </footer>
</body>
</html>
```

### Nested Layouts

Create `layout.nguyen` in any subdirectory. It wraps pages in that directory and all subdirectories:

```
pages/
├── layout.nguyen           # Root layout (all pages)
├── index.nguyen
├── blog/
│   ├── layout.nguyen       # Blog layout (blog pages only)
│   ├── index.nguyen
│   └── [slug].nguyen
└── docs/
    ├── layout.nguyen       # Docs layout (docs pages only)
    └── getting-started.nguyen
```

Layouts compose from shallowest to deepest. A page at `/blog/hello` gets:
1. Root `layout.nguyen` (outermost)
2. Blog `layout.nguyen` (innermost, wraps page content)

### Named Slots

Layouts can define multiple slot regions:

```nguyen
<!-- layout.nguyen -->
<div class="layout">
    <aside>
        <nguyen-slot name="sidebar" />
    </aside>
    <main>
        <nguyen-slot />
    </main>
</div>
```

Pages fill named slots with the `slot` attribute:

```nguyen
<!-- page.nguyen -->
<div slot="sidebar">
    <nav>Sidebar content</nav>
</div>

<article>
    Main content goes into the default slot.
</article>
```

### Explicit Layout Reference

Instead of auto-discovery, reference a layout explicitly in frontmatter:

```nguyen
---
layout = "app/layout"
---

<h1>This page uses a specific layout</h1>
```

## Navigation

### `<nguyen-link>`

Use `<nguyen-link>` for client-side navigation (no full page reload):

```html
<nguyen-link href="/about">About</nguyen-link>
<nguyen-link href="/blog" class="nav-link">Blog</nguyen-link>
```

In SSR output, `<nguyen-link>` renders as a standard `<a>` tag. On the client, it intercepts clicks for SPA-style navigation.

### Programmatic Navigation

In WASM components:

```go
core.Navigate("/blog/new-post")
```

### Active Path Detection

```go
isActive := core.IsActivePath("/blog", false)  // prefix match
isExact := core.IsActivePath("/blog", true)     // exact match
```

## Route Guards

Protect routes with named guards:

```go
// In your app setup
core.RegisterGuard("auth", func(path string) bool {
    return isAuthenticated()
})
```

Reference in frontmatter:

```nguyen
---
guard = "auth"
---

<h1>Protected Page</h1>
```

## Search Params

Access URL query parameters reactively:

```go
params := core.UseSearchParams()
page := params["page"]    // ?page=2
sort := params["sort"]    // ?sort=name
```

The component re-renders when query params change.

## Special Files

| File | Purpose |
|------|---------|
| `layout.nguyen` | Layout wrapper for directory |
| `404.nguyen` | Custom not-found page |
| `_app.nguyen` | App-level wrapper (internal) |
| Files starting with `_` | Ignored by router |
