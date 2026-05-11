# Configuration

Nguyen.go is configured via `config/nguyen.config.yml`. All settings have sensible defaults.

## Full Reference

```yaml
# Application name
name: my-app

# Application version (used in X-Powered-By header)
version: 1.0.0

# HTTP Server
server:
  host: localhost          # Bind address
  port: 3000              # Listen port
  strict_routing: false   # Trailing slash matters
  case_sensitive: false   # URL case matters

# Rendering
render:
  mode: ssr               # csr | ssr | isr
  stream: false           # Enable streaming SSR (chunked transfer)
  extract_critical_css: false  # Inline critical CSS

# WebAssembly
wasm:
  entry: app/main.go      # WASM entry point
  output: .nguyen/app.wasm  # Output path
  target: wasm            # TinyGo target
  optimize: true          # Enable TinyGo optimizations (-opt=z)

# Hot Module Replacement
hmr:
  enabled: true
  sse: /_nguyen/hmr       # SSE endpoint path
  watch:                  # Directories to watch
    - pages/
    - components/
    - styles/

# Image Optimization
images:
  formats: [webp, avif]   # Output formats
  sizes: [640, 750, 1080, 1920]  # Responsive sizes
  quality: 80             # Compression quality (1-100)
  cache_ttl: 86400        # Cache duration in seconds
  cache_dir: .nguyen/cache/images  # Cache directory
  public_dir: public      # Source images directory
  blur_size: 10           # Blur placeholder size

# HTML Head
head:
  title_template: "%s — My App"  # %s is replaced with page title
  default_title: "My App"
  meta:                   # Default meta tags
    description: "Built with Nguyen.go"
    viewport: "width=device-width, initial-scale=1"
  open_graph:             # Default Open Graph tags
    site_name: "My App"
    type: website
    locale: en_US
  preconnect_origins:     # DNS prefetch
    - "https://fonts.gstatic.com"
  preload_wasm: true      # Add <link rel="preload"> for WASM

# GEO (Generative Engine Optimization)
geo:
  enabled: true
  site_name: "My App"
  site_url: "https://example.com"
  organization_name: "My Org"
  organization_url: "https://example.com"
  same_as:               # Social profiles
    - "https://github.com/my-org"
    - "https://twitter.com/my-org"
  author_type: Organization  # Organization | Person
  default_page_type: Article
  sitemap:
    enabled: true
    default_changefreq: weekly  # always|hourly|daily|weekly|monthly|yearly|never
    default_priority: 0.7       # 0.0 - 1.0
  robots:
    enabled: true
    allow_ai_bots: false  # Allow AI crawlers (GPTBot, etc.)
  llms_txt:
    enabled: true
    max_chars_per_page: 5000  # Max content per page in llms.txt
  cache_ttl: 3600        # GEO response cache in seconds

# Progressive Web App
pwa:
  enabled: true
  short_name: "MyApp"
  description: "My Progressive Web App"
  theme_color: "#6366f1"
  background_color: "#ffffff"
  display: standalone     # standalone | fullscreen | minimal-ui | browser
  orientation: portrait   # portrait | landscape | any
  scope: /
  start_url: /
  icons:
    - src: /icon-192.png
      sizes: 192x192
      type: image/png
    - src: /icon-512.png
      sizes: 512x512
      type: image/png
```

## Defaults

If no config file exists, Nguyen.go uses these defaults:

| Setting | Default |
|---------|---------|
| `server.host` | `localhost` |
| `server.port` | `3000` |
| `render.mode` | `csr` |
| `render.stream` | `false` |
| `wasm.optimize` | `true` |
| `hmr.enabled` | `true` |
| `images.quality` | `80` |
| `geo.enabled` | `true` |
| `pwa.enabled` | `true` |
| `pwa.theme_color` | `#6366f1` |

## Environment-Specific Config

The config file path can be overridden via CLI:

```bash
nguyen dev --config config/dev.yml
nguyen start --config config/prod.yml
```

## Validation

The config loader validates:
- `server.port` must be 1-65535
- `render.mode` must be `csr`, `ssr`, or `isr`
- Invalid YAML syntax is reported with file path and error

## Programmatic Access

When using Nguyen.go as a library:

```go
import "nguyen.go/internal/config"

cfg, err := config.Load("config/nguyen.config.yml")
if err != nil {
    cfg = config.DefaultConfig()
}

// Access settings
fmt.Println(cfg.Server.Address())  // "localhost:3000"
fmt.Println(cfg.Render.Mode)       // "ssr"
fmt.Println(cfg.PWA.ThemeColor)    // "#6366f1"
```
