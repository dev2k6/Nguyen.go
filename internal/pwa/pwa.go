package pwa

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dev2k6/Nguyen.go/internal/config"
)

// ====================================================================
// Web App Manifest
// ====================================================================

// Manifest represents a standard Web App Manifest (manifest.json)
type Manifest struct {
	Name            string `json:"name"`
	ShortName       string `json:"short_name"`
	Description     string `json:"description,omitempty"`
	StartURL        string `json:"start_url"`
	Display         string `json:"display"`
	Orientation     string `json:"orientation,omitempty"`
	BackgroundColor string `json:"background_color,omitempty"`
	ThemeColor      string `json:"theme_color,omitempty"`
	Scope           string `json:"scope,omitempty"`
	Icons           []Icon `json:"icons,omitempty"`
	Lang            string `json:"lang,omitempty"`
}

// Icon represents a manifest icon entry
type Icon struct {
	Src   string `json:"src"`
	Sizes string `json:"sizes,omitempty"`
	Type  string `json:"type,omitempty"`
}

// GenerateManifest creates a Web App Manifest from configuration.
func GenerateManifest(cfg *config.NguyenConfig) (*Manifest, error) {
	pwa := cfg.PWA
	if !pwa.Enabled {
		return nil, fmt.Errorf("PWA is disabled in config")
	}

	m := &Manifest{
		Name:            cfg.Name,
		ShortName:       pwa.ShortName,
		Description:     pwa.Description,
		StartURL:        pwa.StartURL,
		Display:         pwa.Display,
		Orientation:     pwa.Orientation,
		BackgroundColor: pwa.BackgroundColor,
		ThemeColor:      pwa.ThemeColor,
		Scope:           pwa.Scope,
		Lang:            "en",
	}

	for _, icon := range pwa.Icons {
		m.Icons = append(m.Icons, Icon{
			Src:   icon.Src,
			Sizes: icon.Sizes,
			Type:  icon.Type,
		})
	}

	return m, nil
}

// WriteManifest writes the manifest.json to the output directory.
func WriteManifest(cfg *config.NguyenConfig, outputDir string) error {
	m, err := GenerateManifest(cfg)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("pwa: cannot marshal manifest: %w", err)
	}

	path := filepath.Join(outputDir, "manifest.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("pwa: cannot write manifest: %w", err)
	}

	return nil
}

// ====================================================================
// Service Worker
// ====================================================================

// GenerateServiceWorker creates a JavaScript service worker string.
// It caches static assets and implements a stale-while-revalidate
// strategy for navigation requests.
func GenerateServiceWorker(cfg *config.NguyenConfig, assetList []string) string {
	cacheName := fmt.Sprintf("nguyen-cache-%s", cfg.Version)
	var sb strings.Builder

	sb.WriteString("// Nguyen.go Service Worker\n")
	sb.WriteString("// Auto-generated during build — do not edit manually.\n\n")

	sb.WriteString("const CACHE_NAME = '")
	sb.WriteString(cacheName)
	sb.WriteString("';\n")

	sb.WriteString("const STATIC_ASSETS = [\n")
	sb.WriteString("  '/styles/output.css',\n")
	sb.WriteString("  '/bridge/nguyen_bridge.js',\n")
	for _, asset := range assetList {
		if strings.HasSuffix(asset, ".css") || strings.HasSuffix(asset, ".js") || strings.HasSuffix(asset, ".wasm") {
			sb.WriteString(fmt.Sprintf("  %q,\n", asset))
		}
	}
	sb.WriteString("];\n\n")

	sb.WriteString(`// Install: cache static assets
self.addEventListener('install', function(event) {
  event.waitUntil(
    caches.open(CACHE_NAME).then(function(cache) {
      return cache.addAll(STATIC_ASSETS);
    }).then(function() {
      return self.skipWaiting();
    })
  );
});

// Activate: clean old caches
self.addEventListener('activate', function(event) {
  event.waitUntil(
    caches.keys().then(function(cacheNames) {
      return Promise.all(
        cacheNames.filter(function(name) {
          return name !== CACHE_NAME;
        }).map(function(name) {
          return caches.delete(name);
        })
      );
    }).then(function() {
      return self.clients.claim();
    })
  );
});

// Fetch: stale-while-revalidate for navigation, cache-first for static assets
self.addEventListener('fetch', function(event) {
  var req = event.request;
  var url = new URL(req.url);

  // Skip non-GET requests
  if (req.method !== 'GET') {
    return;
  }

  // Skip development / HMR endpoints
  if (url.pathname.startsWith('/_nguyen/')) {
    return;
  }

  // Navigation requests (HTML pages)
  if (req.mode === 'navigate') {
    event.respondWith(
      caches.match(req).then(function(response) {
        var fetchPromise = fetch(req).then(function(networkResponse) {
          if (networkResponse && networkResponse.status === 200) {
            var clone = networkResponse.clone();
            caches.open(CACHE_NAME).then(function(cache) {
              cache.put(req, clone);
            });
          }
          return networkResponse;
        }).catch(function() {
          return response;
        });
        return response || fetchPromise;
      })
    );
    return;
  }

  // Static assets: cache-first, fallback to network
  if (STATIC_ASSETS.indexOf(url.pathname) !== -1 ||
      url.pathname.endsWith('.css') ||
      url.pathname.endsWith('.js') ||
      url.pathname.endsWith('.wasm')) {
    event.respondWith(
      caches.match(req).then(function(response) {
        if (response) {
          return response;
        }
        return fetch(req).then(function(networkResponse) {
          if (networkResponse && networkResponse.status === 200) {
            var clone = networkResponse.clone();
            caches.open(CACHE_NAME).then(function(cache) {
              cache.put(req, clone);
            });
          }
          return networkResponse;
        });
      })
    );
    return;
  }
});
`)

	return sb.String()
}

// WriteServiceWorker writes the generated service worker to sw.js.
func WriteServiceWorker(cfg *config.NguyenConfig, outputDir string, assetList []string) error {
	sw := GenerateServiceWorker(cfg, assetList)
	path := filepath.Join(outputDir, "sw.js")
	if err := os.WriteFile(path, []byte(sw), 0644); err != nil {
		return fmt.Errorf("pwa: cannot write service worker: %w", err)
	}
	return nil
}

// GenerateSWRegister returns a small inline <script> that registers
// the service worker. It is injected into the HTML shell.
func GenerateSWRegister() string {
	return `<script>
(function() {
  if ('serviceWorker' in navigator) {
    window.addEventListener('load', function() {
      navigator.serviceWorker.register('/sw.js')
        .then(function(reg) { console.log('[Nguyen.go PWA] SW registered:', reg.scope); })
        .catch(function(err) { console.warn('[Nguyen.go PWA] SW registration failed:', err); });
    });
  }
})();
</script>`
}

// WriteAll generates manifest.json and sw.js in the output directory.
func WriteAll(cfg *config.NguyenConfig, outputDir string, assetList []string) error {
	if !cfg.PWA.Enabled {
		return nil
	}

	if err := WriteManifest(cfg, outputDir); err != nil {
		return err
	}

	if err := WriteServiceWorker(cfg, outputDir, assetList); err != nil {
		return err
	}

	return nil
}
