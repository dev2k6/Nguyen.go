# Rendering Modes

Nguyen.go supports three rendering modes, configurable globally or per-page.

## Overview

| Mode | Description | Best For |
|------|-------------|----------|
| **SSR** | Server-Side Rendering | SEO, dynamic content, fast TTFB |
| **ISR** | Incremental Static Regeneration | Cached pages with background revalidation |
| **CSR** | Client-Side Rendering | Highly interactive apps, WASM-heavy |

## SSR (Server-Side Rendering)

Pages are rendered on the server for every request. The HTML is sent complete with all content, making it immediately indexable by search engines.

### Configuration

```yaml
# config/nguyen.config.yml
render:
  mode: ssr
```

### How It Works

1. Request arrives at the server
2. Nguyen.go parses the `.nguyen` file
3. Go frontmatter is evaluated (state initialization, metadata extraction)
4. Template is rendered with `{variable}` interpolation
5. Layout composition is applied
6. Meta tags and hydration markers are injected
7. Complete HTML is sent to the client

### Hydration

SSR pages include a hydration script that:
- Reads `data-nguyen-text` attributes to initialize client state
- Binds `data-nguyen-{event}` attributes for event delegation
- Enables reactive updates without full page reload

### Streaming SSR

Enable chunked transfer for faster Time to First Byte:

```yaml
render:
  mode: ssr
  stream: true
```

With streaming, the server sends HTML in chunks as it renders. The browser can start parsing and displaying content before the full response is complete.

## ISR (Incremental Static Regeneration)

Pages are rendered once, cached, and served from cache. When the cache expires, the page is revalidated in the background while serving the stale version.

### Configuration

```yaml
render:
  mode: isr
```

### Per-Page Revalidation

Set revalidation time in the page frontmatter:

```nguyen
---
export const revalidate = 60  // seconds
---

<h1>This page revalidates every 60 seconds</h1>
```

### How It Works

1. First request: page is rendered (SSR) and cached
2. Subsequent requests within TTL: served from cache (`X-Nguyen-ISR: fresh`)
3. Request after TTL expires: stale page served immediately, background revalidation triggered (`X-Nguyen-ISR: stale`)
4. Next request gets the fresh version

### Cache Headers

ISR automatically sets appropriate cache headers:

- Fresh: `Cache-Control: s-maxage=3600, stale-while-revalidate=86400`
- Stale: `Cache-Control: s-maxage=60, stale-while-revalidate=3600`
- Miss: `Cache-Control: s-maxage={revalidate}, stale-while-revalidate=86400`

## CSR (Client-Side Rendering)

Templates are sent as-is to the client. All rendering is handled by the WASM runtime in the browser.

### Configuration

```yaml
render:
  mode: csr
```

### How It Works

1. Server sends the raw HTML template
2. Browser loads the WASM binary (`app.wasm` or per-route chunks)
3. WASM runtime mounts the virtual DOM
4. All state management and rendering happens client-side

### When to Use CSR

- Highly interactive applications (dashboards, editors)
- Pages that don't need SEO
- When you want full WASM-powered reactivity

## Template Interpolation

All rendering modes support `{variable}` interpolation:

```nguyen
---
count, setCount := core.UseGlobalState("count", 0)
---

<p>Current count: {count}</p>
```

In SSR/ISR mode, this renders as:
```html
<p>Current count: <span data-nguyen-text="count">0</span></p>
```

The `data-nguyen-text` attribute enables client-side reactivity after hydration.

## Event Bindings

Event handlers work across all modes:

```html
<button @click="increment()">+1</button>
```

In SSR mode, this renders as:
```html
<button data-nguyen-click="increment">+1</button>
```

The hydration script sets up event delegation to handle these.

## Meta Tags & SEO

Define metadata in frontmatter:

```nguyen
---
export const metadata = {
    title: "My Page Title",
    description: "Page description for SEO",
    og:image: "/og-image.png",
    og:type: "article",
}
---
```

SSR injects these into `<head>`:
- `<title>` tag
- `<meta name="description">`
- Open Graph `<meta property="og:*">` tags
- `X-Powered-By` header

## Critical CSS

Extract and inline critical CSS for faster rendering:

```yaml
render:
  extract_critical_css: true
```

This inlines above-the-fold CSS directly in the HTML and defers the rest.

## Security Headers

All rendering modes automatically include security headers:

- `Strict-Transport-Security: max-age=63072000; includeSubDomains; preload`
- `X-Frame-Options: DENY`
- `X-Content-Type-Options: nosniff`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Permissions-Policy: camera=(), microphone=(), geolocation=()`
