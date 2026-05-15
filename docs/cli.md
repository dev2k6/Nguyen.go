# CLI Reference

The `nguyen` CLI provides all commands needed to develop, build, and deploy Nguyen.go applications.

## Global Flags

| Flag | Description |
|------|-------------|
| `--root <path>` | Project root directory (chdir before running) |
| `--version` | Print version |
| `--help` | Print help |

## nguyen create

Scaffold a new project from the built-in template.

```bash
nguyen create <project-name> [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--name <name>` | directory name | Project name |
| `--tailwind` | `false` | Set up Tailwind CSS via standalone CLI |

### What It Creates

- `pages/` with index, about, and 404 pages
- `layout.gox` root layout
- `styles/` with CSS
- `api/` for API routes
- `config/nguyen.config.yml`
- `go.mod` and `go.sum`

### Tailwind Setup

With `--tailwind`, the CLI:
1. Downloads the Tailwind CSS standalone binary for your OS/arch
2. Creates `styles/input.css` with `@import "tailwindcss"`
3. Creates `tailwind.config.js`
4. Builds initial `styles/output.css`

No npm or Node.js required.

## nguyen dev

Start the development server with Hot Module Replacement.

```bash
nguyen dev [flags]
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--port` | `-p` | `3000` | Port to listen on |
| `--pages` | | `pages` | Pages directory |
| `--config` | `-c` | `config/nguyen.config.yml` | Config file path |

### Features

- SSE-based HMR (CSS hot reload, full page reload for templates)
- File watcher on pages, components, and styles
- Tailwind CSS watcher (if `bin/tailwindcss` exists)
- ISR cache with disk persistence
- GEO routes (sitemap.xml, robots.txt, llms.txt)
- PWA manifest and service worker
- SPA navigation endpoint

### Example

```bash
nguyen dev --port 8080 --pages src/pages
```

## nguyen build

Compile `.gox` files into optimized production output.

```bash
nguyen build [flags]
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--output` | `-o` | `.nguyen` | Output directory |
| `--pages` | | `pages` | Pages directory |
| `--optimize` | | `true` | Enable TinyGo optimizations |
| `--per-route` | | `true` | Code-split per route |
| `--analyze` | | `false` | Generate bundle size report |
| `--no-wasm` | | `false` | Skip WASM compilation (SSR-only) |

### Build Steps

1. **Parse** — Validate and transpile all `.gox` files
2. **Compile** — Generate Go source for each component
3. **WASM** — Compile to WebAssembly via TinyGo (unless `--no-wasm`)
4. **PWA** — Generate manifest.json and sw.js
5. **Static HTML** — Pre-render all static routes
6. **Assets** — Copy styles and public directories

### Per-Route Code Splitting

With `--per-route` (default), each page gets its own `.wasm` chunk:

```
.nguyen/
├── chunks/
│   ├── index.wasm
│   ├── about.wasm
│   ├── blog__slug.wasm
│   └── manifest.json
├── bridge.js
├── index.html
├── about/index.html
└── manifest.json
```

### Bundle Analyzer

```bash
nguyen build --analyze
```

Generates `.nguyen/bundle-report.html` with a visual breakdown of WASM chunk sizes.

### SSR-Only Build

```bash
nguyen build --no-wasm
```

Skips TinyGo compilation entirely. Pages are served as pre-rendered HTML with the vanilla JS hydration script (no WASM required).

## nguyen start

Start the production server from build output.

```bash
nguyen start [flags]
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--port` | `-p` | `3000` | Port to listen on |
| `--dir` | `-d` | `.nguyen` | Build output directory |
| `--config` | `-c` | `config/nguyen.config.yml` | Config file path |

### Features

- Serves pre-built static HTML
- WASM binary serving with immutable cache headers
- Brotli/gzip compression
- ETag support
- Rate limiting (600 req/min/IP)
- Security headers
- SPA fallback (serves index.html for unmatched routes)
- Path traversal protection

### Example

```bash
nguyen build
nguyen start --port 8080
```

## nguyen export

Pre-render all pages to static HTML for CDN deployment.

```bash
nguyen export [flags]
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--output` | `-o` | `.nguyen/static` | Output directory |
| `--pages` | | `pages` | Pages directory |
| `--config` | `-c` | `config/nguyen.config.yml` | Config file path |

### Output

- Static HTML for every non-dynamic route
- `sitemap.xml` (if GEO enabled)
- `robots.txt` (if GEO enabled)
- `llms.txt` (if GEO enabled)

### Limitations

Dynamic routes (`[slug]`, `[...path]`) cannot be statically exported without known parameters. They are skipped during export.

### Example

```bash
nguyen export --output dist
# Deploy dist/ to any static host
```

## nguyen check

Validate `.gox` file syntax without building.

```bash
nguyen check [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--pages` | `pages` | Pages directory to check |

### Checks Performed

- Proper frontmatter delimiters (`---`)
- Valid Go code in frontmatter
- Template syntax (interpolation, event bindings)
- Dynamic route parameter naming
- Mismatched curly braces
- Unknown event bindings

### Output

```
  ● index.gox          ✓ PASS
    Package: page_index  States: 1  Events: 2
  ● about.gox          ✓ PASS
    Package: page_about  States: 0  Events: 0
  ● blog/[slug].gox    ✓ PASS
    Package: page_slug   States: 0  Events: 0

  ────────────────────────────────────────
  Results:  3 passed
  ────────────────────────────────────────
```

### Exit Codes

- `0` — All files valid
- `1` — One or more files have errors

## nguyen doctor

Diagnose the local toolchain and project layout. See [Tooling](./tooling.md#nguyen-doctor) for full output.

```bash
nguyen doctor
```

Checks Go version, TinyGo binary, Tailwind binary, project directories
(`pages/`, `public/`, `styles/`), config file presence and git status.
Exits non-zero only when a hard prerequisite is unmet.

## nguyen vet

Run static analysis on `.gox` files. See [Tooling](./tooling.md#nguyen-vet).

```bash
nguyen vet [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--pages` | `pages` | Pages directory |
| `--strict` | `false` | Treat warnings as errors |

Errors fail vet (exit 1). Warnings do not unless `--strict`. Built-in
rules cover context propagation, time.Sleep usage, goroutine
cancellation, hardcoded secrets, fmt.Print* and event handler bindings.

## nguyen generate

Code generation utilities. Sub-commands:

### nguyen generate routes

```bash
nguyen generate routes [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--pages` | `pages` | Pages directory |
| `--out` | `routes/routes.gen.go` | Output path |
| `--package` | `routes` | Generated package name |

Emits typed accessors so dynamic routes are checked at compile time.
See [Tooling](./tooling.md#nguyen-generate-routes).

## nguyen dist

Cross-compile reproducible single-binary releases.

```bash
nguyen dist [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--targets` | current GOOS/GOARCH | Comma-separated GOOS/GOARCH pairs |
| `--output` | `dist` | Output directory |
| `--name` | `app` | Binary name |
| `--main` | `./cmd/server` | Path to main package |
| `--reproducible` | `true` | Strip path/buildid/timestamps |
| `--ldflags` | `""` | Extra ldflags |

When reproducible is enabled, dist sets `-trimpath`, `-buildvcs=false`,
strips symbols and the build ID, and disables CGo. Two builds from the
same source tree on different machines produce byte-identical output.
See [Tooling](./tooling.md#nguyen-dist).
