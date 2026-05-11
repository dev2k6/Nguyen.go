# Nguyen.go

A full-stack Go web framework for building modern web applications. Nguyen.go compiles Go to WebAssembly (via TinyGo) for the frontend, while the backend runs natively on Go using Fiber. Think Next.js for Go — with server-side rendering (SSR), incremental static regeneration (ISR), islands architecture, and a React-style virtual DOM reconciler.

## Features

### Core
- **Go + TinyGo WASM** — Write frontend components in Go, compile to WebAssembly
- **React-style reconciler** — Virtual DOM diffing with keyed reconciliation
- **Template syntax** — `.nguyen` files with Go frontmatter and HTML templates
- **SSR / ISR / CSR** — Server-side rendering, Incremental Static Regeneration, or Client-Side Rendering per page
- **Streaming SSR** — Transfer-Encoding: chunked responses for faster Time to First Byte
- **Hot Module Replacement** — SSE-based HMR for CSS and full page reloads

### Templates
- **Control flow directives** — `<ng-if>`, `<ng-else-if>`, `<ng-else>`, `<ng-for>`
- **Named slots** — Multiple slot regions in layouts
- **Scoped CSS** — Component-scoped styles via `<style scoped>`
- **Variable interpolation** — `{stateVar}` in templates with reactive hydration

### Routing
- **File-system routing** — `pages/index.nguyen` → `/`, `pages/about.nguyen` → `/about`
- **Nested layouts** — Auto-discovered `layout.nguyen` per directory hierarchy
- **Dynamic routes** — `pages/blog/[slug].nguyen` → `/blog/:slug`
- **Catch-all routes** — `pages/[...path].nguyen` → wildcard matching
- **Optional params** — `pages/[[...path]].nguyen` → optional catch-all
- **Route guards** — Named guard registration with `RegisterGuard`
- **Search params** — `UseSearchParams()` with reactive updates

### Data Fetching
- **`UsePageData`** — Server-injected props via SSR
- **`UseQuery`** / **`UseMutation`** — Async data fetching with caching
- **`UseForm`** — Form state management with per-field validators
- **`FetchJSON`** — HTTP client helper

### State & Hooks
- **`UseState`** — Reactive state hook
- **`UseEffect`** / **`UseLayoutEffect`** — Side effect hooks
- **`UseMemo`** / **`UseCallback`** — Performance optimization hooks
- **`UseLocalStorage`** / **`UseSessionStorage`** — Cross-tab synced browser storage
- **`UseGlobalState`** — App-level shared state

### Component DX
- **`Suspense`** — Async component boundary with fallback
- **`ErrorBoundary`** — Panic recovery in child components
- **`NguyenTransition`** — CSS enter/leave animations
- **`Memo`** — Shallow props comparison for memoization
- **`NguyenLink`** — Active path-aware link component with SPA navigation

### Build & Runtime
- **Per-route code splitting** — Separate `.wasm` chunks per page + shared core
- **Islands architecture** — Partial hydration of interactive components
- **PWA support** — Auto-generated `manifest.json` + `sw.js` with offline caching
- **Image optimization** — Built-in `/_nguyen/image` endpoint with WebP/AVIF conversion
- **Bundle analyzer** — HTML report of WASM chunk sizes
- **GEO / SEO** — Sitemap, robots.txt, llms.txt, Open Graph, JSON-LD, FAQ schema, breadcrumbs

## Requirements

- **Go** 1.25+
- **TinyGo** (optional, for WASM compilation — SSR works without it)

## Installation

```bash
go install github.com/dev2k6/Nguyen.go@latest
```

Or build from source:

```bash
git clone https://github.com/dev2k6/Nguyen.go.git
cd nguyen.go
go build -o nguyen ./cmd/nguyen/
```

## Quick Start

```bash
# Create a new project
nguyen create my-app

# With Tailwind CSS (standalone CLI, no npm)
nguyen create my-app --tailwind

# Enter the project
cd my-app

# Start the dev server with HMR
nguyen dev

# Open http://localhost:3000
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `nguyen create <name>` | Scaffold a new project from template |
| `nguyen dev` | Start dev server with HMR and file watching |
| `nguyen build` | Compile `.nguyen` files to optimized WASM + static HTML |
| `nguyen start` | Start production server from build output |
| `nguyen export` | Pre-render all pages to static HTML for CDN deployment |
| `nguyen check` | Validate `.nguyen` file syntax without building |

### Common Flags

```bash
nguyen dev --port 8080              # Custom port
nguyen dev --pages src/pages        # Custom pages directory
nguyen dev --config my.config.yml   # Custom config file

nguyen build --per-route            # Code-split per route (default)
nguyen build --no-wasm              # SSR-only build (skip TinyGo)
nguyen build --analyze              # Generate bundle size report

nguyen export --output dist         # Custom output directory

nguyen start --port 8080            # Custom port
nguyen start --dir .nguyen          # Build directory to serve
```

## Project Structure

```
my-app/
├── pages/                  # File-system routes (.nguyen files)
│   ├── index.nguyen        # → /
│   ├── about.nguyen        # → /about
│   ├── 404.nguyen          # → catch-all fallback
│   ├── layout.nguyen       # Root layout (wraps all pages)
│   └── blog/
│       ├── index.nguyen    # → /blog
│       ├── [slug].nguyen   # → /blog/:slug
│       └── layout.nguyen   # Blog-specific layout
├── api/                    # API route handlers (Go)
│   └── routes.go
├── styles/                 # CSS files
│   ├── input.css           # Tailwind input
│   └── output.css          # Compiled CSS
├── public/                 # Static assets (served at /public/*)
├── config/
│   └── nguyen.config.yml   # Framework configuration
├── go.mod
└── go.sum
```

## Template Syntax (`.nguyen` files)

A `.nguyen` file has two sections separated by `---`:

1. **Frontmatter** — Go code (imports, state, handlers, metadata)
2. **Template** — HTML with reactive interpolation

```nguyen
---
import "nguyen.go/pkg/core"

count, setCount := core.UseState("count", 0)

func increment() {
    setCount(count + 1)
}

func decrement() {
    setCount(count - 1)
}

export const metadata = {
    title: "Counter Example",
    description: "A simple counter built with Nguyen.go",
}
---

<section class="counter">
    <h1>Count: {count}</h1>
    <button @click="increment()">+</button>
    <button @click="decrement()">-</button>
</section>

<style scoped>
.counter {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1rem;
}
</style>
```

### Event Bindings

```html
<button @click="handler()">Click me</button>
<input @input="onInput()" />
<form @submit="onSubmit()">...</form>
```

Supported events: `@click`, `@input`, `@change`, `@submit`, `@keydown`, `@keyup`, `@focus`, `@blur`, `@mouseenter`, `@mouseleave`, `@dblclick`

### Control Flow

```html
<ng-if condition="isLoggedIn">
    <p>Welcome back!</p>
</ng-if>
<ng-else>
    <p>Please log in.</p>
</ng-else>

<ng-for items="posts" item="post" index="i">
    <article>
        <h2>{post.Title}</h2>
        <p>{post.Body}</p>
    </article>
</ng-for>
```

### Layouts

Create a `layout.nguyen` in any directory. It wraps all pages in that directory and below:

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
    <nav>...</nav>
    <main>
        <nguyen-slot />
    </main>
    <footer>...</footer>
</body>
</html>
```

Use `<nguyen-slot />` for the default content slot and `<nguyen-slot name="sidebar" />` for named slots.

## Configuration

Create `config/nguyen.config.yml`:

```yaml
name: my-app
version: 1.0.0

server:
  host: localhost
  port: 3000
  strict_routing: false
  case_sensitive: false

render:
  mode: ssr              # csr | ssr | isr
  stream: true           # enable streaming SSR
  extract_critical_css: true

wasm:
  entry: app/main.go
  output: .nguyen/app.wasm
  target: wasm
  optimize: true

hmr:
  enabled: true
  sse: /_nguyen/hmr
  watch:
    - pages/
    - components/
    - styles/

images:
  formats: [webp, avif]
  sizes: [640, 750, 1080, 1920]
  quality: 80
  cache_ttl: 86400

head:
  title_template: "%s — My App"
  default_title: "My App"
  meta:
    description: "Built with Nguyen.go"
  preload_wasm: true

geo:
  enabled: true
  site_name: "My App"
  site_url: "https://example.com"
  sitemap:
    enabled: true
    default_changefreq: weekly
    default_priority: 0.7
  robots:
    enabled: true
    allow_ai_bots: false
  llms_txt:
    enabled: true
    max_chars_per_page: 5000

pwa:
  enabled: true
  short_name: "MyApp"
  description: "My Progressive Web App"
  theme_color: "#6366f1"
  background_color: "#ffffff"
  display: standalone
  icons:
    - src: /icon-192.png
      sizes: 192x192
      type: image/png
    - src: /icon-512.png
      sizes: 512x512
      type: image/png
```

## Rendering Modes

### SSR (Server-Side Rendering)
Pages are rendered on the server for every request. Best for dynamic content and SEO.

```yaml
render:
  mode: ssr
  stream: true  # optional: stream HTML chunks
```

### ISR (Incremental Static Regeneration)
Pages are cached after first render and revalidated in the background. Set per-page revalidation time:

```nguyen
---
export const revalidate = 60  // revalidate every 60 seconds
---
<h1>This page is cached with ISR</h1>
```

### CSR (Client-Side Rendering)
Templates are sent as-is; WASM handles all rendering on the client. Best for highly interactive apps.

```yaml
render:
  mode: csr
```

## GEO (Generative Engine Optimization)

Nguyen.go includes built-in support for AI-friendly SEO:

```nguyen
---
export const geo = {
    pageType: "Article",
    datePublished: "2026-01-01T00:00:00Z",
    dateModified: "2026-05-10T00:00:00Z",
    author: "Your Name",
    speakable: [".main-content"],
    breadcrumbs: [
        {name: "Home", url: "/"},
        {name: "Blog", url: "/blog"},
        {name: "This Post", url: "/blog/this-post"},
    ],
    faqs: [
        {question: "What is Nguyen.go?", answer: "A full-stack Go web framework."},
    ],
    tags: ["go", "webassembly", "framework"],
}
---
```

Auto-generated endpoints:
- `/sitemap.xml` — XML sitemap with all routes
- `/robots.txt` — Crawler directives
- `/llms.txt` — LLM-friendly site summary
- `/.well-known/ai-bot.txt` — AI bot permissions

## Programmatic Usage

Use Nguyen.go as a library in your own Go application:

```go
package main

import (
    "nguyen.go/pkg/nguyen"
    "github.com/gofiber/fiber/v2"
)

func main() {
    app := nguyen.New(
        nguyen.WithPort(3000),
        nguyen.WithPages("pages"),
        nguyen.WithPublic("public"),
        nguyen.WithStyles("styles"),
        nguyen.WithSetup(func(f *fiber.App) {
            // Add custom Fiber routes/middleware
            f.Get("/api/health", func(c *fiber.Ctx) error {
                return c.JSON(fiber.Map{"status": "ok"})
            })
        }),
    )

    app.Listen(":3000")
}
```

## Deployment

### Single Binary

```bash
nguyen build
go build -o server ./cmd/nguyen/
./server start --dir .nguyen --port 8080
```

### Static Export (CDN)

```bash
nguyen export --output dist
# Upload dist/ to any static host (Netlify, Vercel, Cloudflare Pages, S3)
```

### Docker

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o nguyen ./cmd/nguyen/
RUN ./nguyen build --no-wasm

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/nguyen .
COPY --from=builder /app/.nguyen .nguyen
COPY --from=builder /app/config config
EXPOSE 3000
CMD ["./nguyen", "start"]
```

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      CLI (cobra)                         │
│  create │ dev │ build │ start │ export │ check           │
└────────────────────────┬────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────┐
│                   Internal Packages                       │
├──────────┬──────────┬──────────┬──────────┬─────────────┤
│  parser  │  router  │  render  │ compiler │   server    │
│  (.nguyen│  (file-  │  (SSR,   │  (TinyGo │  (Fiber,   │
│   lexer) │  system) │  stream) │   WASM)  │   HMR)     │
├──────────┼──────────┼──────────┼──────────┼─────────────┤
│  config  │  cache   │   geo    │   pwa    │  optimizer  │
│  (YAML)  │  (ISR)   │  (SEO)  │(manifest)│  (images)   │
└──────────┴──────────┴──────────┴──────────┴─────────────┘
                         │
┌────────────────────────▼────────────────────────────────┐
│                   Public Packages                         │
├─────────────────┬───────────────────────────────────────┤
│    pkg/nguyen   │            pkg/core                    │
│  (App, Options) │  (VNode, Hooks, Reconciler, Router)   │
└─────────────────┴───────────────────────────────────────┘
```

## Editor Support

### Vim/Neovim

Copy `ftdetect/nguyen.vim` to your config:

```bash
# Neovim
cp ftdetect/nguyen.vim ~/.config/nvim/ftdetect/

# Vim
cp ftdetect/nguyen.vim ~/.vim/ftdetect/
```

This enables Go syntax highlighting for `.nguyen` files.

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Run tests (`go test ./...`)
4. Run syntax check on examples (`go run ./cmd/nguyen check --pages examples/hello-nguyen/pages`)
5. Commit your changes
6. Push to the branch
7. Open a Pull Request

## License

MIT

## Author

Thái Nguyên <thainguyen.junior@gmail.com>
