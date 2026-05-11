# Progressive Web App (PWA)

Nguyen.go includes built-in PWA support with auto-generated manifest and service worker.

## Configuration

Enable PWA in `config/nguyen.config.yml`:

```yaml
pwa:
  enabled: true
  short_name: "MyApp"
  description: "My Progressive Web App"
  theme_color: "#6366f1"
  background_color: "#ffffff"
  display: standalone
  orientation: portrait
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

## Generated Files

### manifest.json

Served at `/manifest.json`. Contains app metadata for install prompts:

```json
{
  "name": "My App",
  "short_name": "MyApp",
  "description": "My Progressive Web App",
  "start_url": "/",
  "scope": "/",
  "display": "standalone",
  "orientation": "portrait",
  "theme_color": "#6366f1",
  "background_color": "#ffffff",
  "icons": [
    { "src": "/icon-192.png", "sizes": "192x192", "type": "image/png" },
    { "src": "/icon-512.png", "sizes": "512x512", "type": "image/png" }
  ]
}
```

### sw.js

Service worker at `/sw.js` with:
- Precaching of WASM chunks, bridge JS, and static assets
- Network-first strategy for HTML pages
- Cache-first strategy for static assets
- Offline fallback

## HTML Injection

When PWA is enabled, the build automatically injects:

```html
<!-- In <head> -->
<link rel="manifest" href="/manifest.json">

<!-- Before </body> -->
<script>
if ('serviceWorker' in navigator) {
    navigator.serviceWorker.register('/sw.js');
}
</script>
```

## Display Modes

| Mode | Description |
|------|-------------|
| `standalone` | Looks like a native app (no browser UI) |
| `fullscreen` | Uses entire screen |
| `minimal-ui` | Minimal browser controls |
| `browser` | Standard browser tab |

## Icons

Provide at minimum:
- 192x192 — Home screen icon
- 512x512 — Splash screen icon

Place icons in `public/` directory:

```
public/
├── icon-192.png
└── icon-512.png
```

## Offline Support

The generated service worker caches:
- All WASM chunks (per-route code splitting)
- Bridge JavaScript
- CSS files
- Static assets from `public/`

On subsequent visits, the app works offline using cached resources.

## Dev Mode

In development (`nguyen dev`), PWA endpoints are served dynamically:
- `/manifest.json` — Generated on-the-fly from config
- `/sw.js` — Generated service worker

## Production Build

During `nguyen build`, PWA assets are written to the build directory:
- `.nguyen/manifest.json`
- `.nguyen/sw.js`

The production server (`nguyen start`) serves these as static files.

## Disabling PWA

```yaml
pwa:
  enabled: false
```

When disabled, no manifest link or service worker registration is injected.
