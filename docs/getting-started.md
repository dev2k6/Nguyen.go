# Getting Started

This guide walks you through creating your first Nguyen.go application from scratch.

## Prerequisites

- **Go 1.25+** — [Download Go](https://go.dev/dl/)
- **TinyGo** (optional) — Required only for WASM compilation. SSR mode works without it. [Install TinyGo](https://tinygo.org/getting-started/install/)

## Installation

### From Go Install

```bash
go install github.com/dev2k6/Nguyen.go@latest
```

### From Source

```bash
git clone https://github.com/dev2k6/Nguyen.go.go.git
cd nguyen.go
go build -o nguyen ./cmd/nguyen/
```

Verify the installation:

```bash
nguyen --version
# nguyen version 1.0.0
```

## Create a New Project

```bash
nguyen create my-app
cd my-app
```

This scaffolds a project with:
- `pages/` — File-system routes
- `styles/` — CSS files
- `public/` — Static assets
- `config/nguyen.config.yml` — Framework configuration

### With Tailwind CSS

```bash
nguyen create my-app --tailwind
```

This additionally downloads the Tailwind CSS standalone CLI (no npm required) and sets up `styles/input.css` → `styles/output.css`.

## Project Structure

```
my-app/
├── pages/
│   ├── index.nguyen        # Home page → /
│   ├── about.nguyen        # About page → /about
│   ├── 404.nguyen          # Not found fallback
│   └── layout.nguyen       # Root layout (wraps all pages)
├── api/
│   └── routes.go           # API route handlers
├── styles/
│   ├── input.css           # Tailwind/CSS source
│   └── output.css          # Compiled CSS
├── public/
│   └── favicon.ico         # Static assets
├── config/
│   └── nguyen.config.yml   # Configuration
├── go.mod
└── go.sum
```

## Start Development Server

```bash
nguyen dev
```

This starts:
- HTTP server on `http://localhost:3000`
- Hot Module Replacement (HMR) via SSE
- File watcher on `pages/`, `styles/`, `components/`
- Tailwind CSS watcher (if binary found in `bin/`)

## Your First Page

Create `pages/hello.nguyen`:

```nguyen
---
export const metadata = {
    title: "Hello World",
    description: "My first Nguyen.go page",
}
---

<section class="p-8 text-center">
    <h1 class="text-4xl font-bold">Hello, Nguyen.go!</h1>
    <p class="mt-4 text-gray-600">This page is available at /hello</p>
</section>
```

Visit `http://localhost:3000/hello` to see it.

## Adding Interactivity

Create `pages/counter.nguyen`:

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
    description: "Interactive counter example",
}
---

<section class="p-8 text-center">
    <h1 class="text-2xl font-bold mb-4">Counter: {count}</h1>
    <div class="flex gap-4 justify-center">
        <button @click="decrement()" class="btn">-</button>
        <button @click="increment()" class="btn">+</button>
    </div>
</section>
```

The `{count}` interpolation is reactive — it updates automatically when state changes.

## Build for Production

```bash
# Full build (WASM + static HTML)
nguyen build

# SSR-only build (no TinyGo required)
nguyen build --no-wasm

# Start production server
nguyen start
```

## Static Export

For CDN deployment without a Go server:

```bash
nguyen export --output dist
```

Upload `dist/` to any static host (Netlify, Vercel, Cloudflare Pages, S3).

## Next Steps

- [Routing](./routing.md) — File-system routing, dynamic params, layouts
- [Rendering](./rendering.md) — SSR, ISR, CSR modes
- [Hooks & State](./hooks.md) — UseState, UseEffect, UseGlobalState
- [Components](./components.md) — VNode, Component interface, Islands
- [Configuration](./configuration.md) — nguyen.config.yml reference
- [CLI Reference](./cli.md) — All commands and flags
- [Deployment](./deployment.md) — Docker, CDN, single binary
