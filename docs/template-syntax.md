# Template Syntax

Nguyen.go uses `.nguyen` files — a single-file format combining Go logic with HTML templates.

## File Structure

A `.nguyen` file has two sections separated by `---`:

```nguyen
---
// Go frontmatter: imports, state, handlers, metadata
---

<!-- HTML template: reactive markup -->
```

If no `---` delimiters are present, the entire file is treated as a template.

## Frontmatter

The frontmatter section contains Go code that runs at build/render time:

```nguyen
---
import "nguyen.go/pkg/core"

// State declarations
count, setCount := core.UseGlobalState("count", 0)
name, setName := core.UseGlobalState("name", "World")

// Event handlers
func increment() {
    setCount(count.(int) + 1)
}

func updateName(newName string) {
    setName(newName)
}

// Metadata (extracted by SSR)
export const metadata = {
    title: "My Page",
    description: "Page description",
}

// ISR revalidation time
export const revalidate = 60

// Layout reference
layout = "app/layout"
---
```

## Variable Interpolation

Use `{variableName}` to insert reactive state values:

```html
<h1>Hello, {name}!</h1>
<p>Count: {count}</p>
<span>Status: {status}</span>
```

In SSR mode, these render as hydration-ready spans:
```html
<h1>Hello, <span data-nguyen-text="name">World</span>!</h1>
```

Unknown variables render as empty strings.

## Event Bindings

Bind DOM events with `@event="handler()"`:

```html
<button @click="increment()">Click me</button>
<input @input="onInput()" />
<form @submit="onSubmit()">...</form>
<div @mouseenter="onHover()">Hover me</div>
```

### Supported Events

| Directive | DOM Event |
|-----------|-----------|
| `@click` | click |
| `@dblclick` | dblclick |
| `@input` | input |
| `@change` | change |
| `@submit` | submit |
| `@keydown` | keydown |
| `@keyup` | keyup |
| `@focus` | focus |
| `@blur` | blur |
| `@mouseenter` | mouseenter |
| `@mouseleave` | mouseleave |
| `@mouseover` | mouseover |
| `@mouseout` | mouseout |

## Control Flow

### Conditional Rendering

```html
<ng-if condition="isLoggedIn">
    <p>Welcome back, {username}!</p>
</ng-if>
<ng-else-if condition="isGuest">
    <p>Welcome, guest!</p>
</ng-else-if>
<ng-else>
    <p>Please log in.</p>
</ng-else>
```

### List Rendering

```html
<ul>
    <ng-for items="posts" item="post" index="i">
        <li>
            <h3>{post.Title}</h3>
            <p>{post.Body}</p>
        </li>
    </ng-for>
</ul>
```

## Scoped CSS

Add `<style scoped>` for component-scoped styles:

```nguyen
---
export const metadata = { title: "Styled Page" }
---

<div class="card">
    <h1>Hello</h1>
    <p>This is scoped</p>
</div>

<style scoped>
.card {
    padding: 2rem;
    border: 1px solid #eee;
    border-radius: 8px;
}
.card h1 {
    color: #333;
}
</style>
```

Scoped CSS generates unique attribute selectors so styles don't leak to other components.

## Metadata

Define page metadata for SEO:

```nguyen
---
export const metadata = {
    title: "Blog Post Title",
    description: "A brief description of this page",
    og:image: "/images/og-blog.png",
    og:type: "article",
    author: "Thái Nguyên",
}
---
```

SSR injects these as `<title>` and `<meta>` tags in `<head>`.

## GEO Metadata

Define structured data for AI/search engines:

```nguyen
---
export const geo = {
    pageType: "Article",
    datePublished: "2026-01-15T00:00:00Z",
    dateModified: "2026-05-10T00:00:00Z",
    author: "Thái Nguyên",
    speakable: [".main-content", ".summary"],
    breadcrumbs: [
        {name: "Home", url: "/"},
        {name: "Blog", url: "/blog"},
        {name: "This Post", url: "/blog/this-post"},
    ],
    faqs: [
        {question: "What is Nguyen.go?", answer: "A full-stack Go web framework."},
        {question: "Does it need Node.js?", answer: "No, it's pure Go."},
    ],
    howToSteps: [
        {name: "Install", text: "Run go install github.com/dev2k6/Nguyen.go@latest"},
        {name: "Create", text: "Run nguyen create my-app"},
        {name: "Develop", text: "Run nguyen dev"},
    ],
    tags: ["go", "webassembly", "framework"],
    images: ["/og-image.png"],
    sameAs: ["https://github.com/dev2k6/Nguyen.go"],
}
---
```

## Links

Use `<nguyen-link>` for SPA navigation:

```html
<nguyen-link href="/about">About</nguyen-link>
<nguyen-link href="/blog" class="nav-link active">Blog</nguyen-link>
```

In SSR output, renders as `<a>` tags. On the client, intercepts clicks for client-side navigation without full page reload.

## Slots

### Default Slot

In layouts, `<nguyen-slot />` marks where page content is inserted:

```html
<main>
    <nguyen-slot />
</main>
```

### Named Slots

Define multiple insertion points:

```html
<!-- layout.nguyen -->
<div class="layout">
    <aside><nguyen-slot name="sidebar" /></aside>
    <main><nguyen-slot /></main>
</div>
```

Fill named slots from pages:

```html
<!-- page.nguyen -->
<nav slot="sidebar">Sidebar content</nav>
<article>Main content</article>
```

## Head Placeholder

Use `<nguyen-head />` in layouts to mark where meta tags are injected:

```html
<head>
    <meta charset="UTF-8">
    <link rel="stylesheet" href="/styles/output.css">
    <nguyen-head />
</head>
```

SSR replaces this with generated `<title>`, `<meta>`, and Open Graph tags.

## Layout Declaration

Reference a layout explicitly:

```nguyen
---
layout = "app/layout"
---
```

Or rely on auto-discovery: `layout.nguyen` in the same or parent directory applies automatically.

## ISR Revalidation

Set per-page cache revalidation time:

```nguyen
---
export const revalidate = 300  // revalidate every 5 minutes
---
```

Only applies when `render.mode` is `isr` in config.

## Route Guards

Protect a page with a named guard:

```nguyen
---
guard = "auth"
---

<h1>Protected Content</h1>
```

The guard must be registered in your application code.
