# Nguyen.go Documentation

Welcome to the Nguyen.go documentation. Nguyen.go is a full-stack Go web framework that compiles to WebAssembly for the frontend while running natively on Go for the backend.

## Table of Contents

### Getting Started
- [Getting Started](./getting-started.md) — Installation, project setup, first page

### Core Concepts
- [Template Syntax](./template-syntax.md) — `.gox` file format, interpolation, events, control flow
- [Routing](./routing.md) — File-system routing, dynamic params, layouts, navigation
- [Rendering](./rendering.md) — SSR, ISR, CSR modes, streaming, hydration
- [Hooks & State](./hooks.md) — UseState, UseEffect, UseMemo, global state, scheduler
- [Components](./components.md) — VNode, Component interface, Memo, Suspense, ErrorBoundary, Islands

### Features
- [GEO (SEO for AI)](./geo.md) — Sitemap, robots.txt, llms.txt, JSON-LD, structured data
- [PWA](./pwa.md) — Progressive Web App, manifest, service worker, offline
- [Image Optimization](./images.md) — On-the-fly conversion, responsive images, caching

### Reference
- [Configuration](./configuration.md) — Full `nguyen.config.yml` reference
- [CLI Reference](./cli.md) — All commands and flags
- [Deployment](./deployment.md) — Docker, CDN, single binary, reverse proxy
- [Editor & IDE Setup](./editor-setup.md) — VS Code, JetBrains, Zed, Vim, Helix, Sublime
- [AI Agent Reference](./agent-reference.md) — Syntax guide for AI coding assistants

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
│  (.gox│  (file-  │  (SSR,   │  (TinyGo │  (Fiber,   │
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

## Key Design Decisions

- **Go all the way** — Backend (Fiber), frontend (TinyGo → WASM), tooling (CLI). One language, one ecosystem.
- **File-system routing** — No manual route registration. Drop a file, get a route.
- **Islands by default** — Ship zero JS unless a component opts into interactivity.
- **SSR-first** — Pages are server-rendered for SEO and fast TTFB. WASM hydrates interactivity.
- **Single binary** — `go build` produces one executable. No Node.js, no Docker required.
- **React-familiar** — Hooks, virtual DOM, reconciler, lanes scheduler. If you know React, you know Nguyen.go.

## Quick Links

| I want to... | Go to |
|--------------|-------|
| Create a new project | [Getting Started](./getting-started.md) |
| Understand `.gox` files | [Template Syntax](./template-syntax.md) |
| Add a new page | [Routing](./routing.md) |
| Make a page interactive | [Hooks & State](./hooks.md) |
| Deploy to production | [Deployment](./deployment.md) |
| Optimize for search/AI | [GEO](./geo.md) |
| Configure the framework | [Configuration](./configuration.md) |
