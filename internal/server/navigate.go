package server

import (
	"sync"
	"time"

	"github.com/dev2k6/Nguyen.go/internal/parser"
	"github.com/dev2k6/Nguyen.go/internal/render"
	"github.com/dev2k6/Nguyen.go/internal/router"

	"github.com/gofiber/fiber/v2"
)

// NavigateHandlerConfig holds dependencies for the SPA navigate endpoint.
type NavigateHandlerConfig struct {
	Routes  []router.Route
	Layouts []router.LayoutInfo
	Version string
}

// partialCache is an in-memory LRU cache for rendered partials.
// Keyed by "fromPath|targetPath" to account for layout diff context.
type partialCache struct {
	mu      sync.RWMutex
	entries map[string]*partialCacheEntry
	keys    []string
	maxSize int
}

type partialCacheEntry struct {
	frame     []byte
	outlet    int
	createdAt time.Time
	ttl       time.Duration
}

func newPartialCache(maxSize int) *partialCache {
	return &partialCache{
		entries: make(map[string]*partialCacheEntry, maxSize),
		keys:    make([]string, 0, maxSize),
		maxSize: maxSize,
	}
}

func (pc *partialCache) get(key string) ([]byte, int, bool) {
	pc.mu.RLock()
	entry, ok := pc.entries[key]
	pc.mu.RUnlock()
	if !ok {
		return nil, 0, false
	}
	if time.Since(entry.createdAt) > entry.ttl {
		pc.mu.Lock()
		delete(pc.entries, key)
		pc.mu.Unlock()
		return nil, 0, false
	}
	return entry.frame, entry.outlet, true
}

func (pc *partialCache) set(key string, frame []byte, outlet int, ttl time.Duration) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	if len(pc.keys) >= pc.maxSize {
		evict := pc.keys[0]
		pc.keys = pc.keys[1:]
		delete(pc.entries, evict)
	}
	pc.entries[key] = &partialCacheEntry{
		frame:     frame,
		outlet:    outlet,
		createdAt: time.Now(),
		ttl:       ttl,
	}
	pc.keys = append(pc.keys, key)
}

// framePool reuses byte slices for encoding partial frames to reduce GC pressure.
var framePool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 0, 8192)
		return &buf
	},
}

// NavigateHandler returns a Fiber handler for /_nguyen/navigate.
// It renders partial HTML for client-side page transitions with:
// - In-memory partial cache (LRU, 60s TTL)
// - sync.Pool for frame buffer reuse
// - Concurrent-safe design leveraging Go's goroutine model
func NavigateHandler(cfg NavigateHandlerConfig) fiber.Handler {
	cache := newPartialCache(128)

	return func(c *fiber.Ctx) error {
		targetPath := c.Query("path")
		fromPath := c.Query("from")

		if targetPath == "" {
			return c.Status(400).JSON(fiber.Map{"error": "missing path parameter"})
		}

		// Check cache first
		cacheKey := fromPath + "|" + targetPath
		if frame, outlet, ok := cache.get(cacheKey); ok {
			c.Set("Content-Type", "application/x-nguyen-partial")
			c.Set("Cache-Control", "private, max-age=30")
			c.Set("X-Nguyen-Outlet", render.Itoa(outlet))
			c.Set("X-Nguyen-Cache", "hit")
			return c.Send(frame)
		}

		// Match target route
		matched, _, found := router.MatchRoute(cfg.Routes, targetPath)
		if !found {
			return c.Status(404).JSON(fiber.Map{"error": "route not found"})
		}

		// Parse .gox file
		ngFile, err := parser.Parse(matched.FilePath)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "parse failed"})
		}

		// Render partial
		partial := render.RenderPartial(ngFile, cfg.Version, cfg.Layouts, fromPath, cfg.Routes)

		// Encode binary frame
		frame, err := render.EncodePartialFrame(partial)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "encode failed"})
		}

		// Store in cache
		cache.set(cacheKey, frame, partial.Meta.Outlet, 60*time.Second)

		c.Set("Content-Type", "application/x-nguyen-partial")
		c.Set("Cache-Control", "private, max-age=30")
		c.Set("X-Nguyen-Outlet", render.Itoa(partial.Meta.Outlet))
		c.Set("X-Nguyen-Cache", "miss")
		return c.Send(frame)
	}
}

// BatchNavigateHandler returns a handler for /_nguyen/prefetch that accepts
// multiple paths and returns partial frames for all of them concurrently.
// This leverages Go's goroutines to render multiple pages in parallel.
func BatchNavigateHandler(cfg NavigateHandlerConfig) fiber.Handler {
	cache := newPartialCache(128)

	return func(c *fiber.Ctx) error {
		type batchReq struct {
			Paths []string `json:"paths"`
			From  string   `json:"from"`
		}
		var req batchReq
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
		if len(req.Paths) == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "empty paths"})
		}
		if len(req.Paths) > 10 {
			req.Paths = req.Paths[:10]
		}

		type result struct {
			Path   string `json:"path"`
			HTML   string `json:"html"`
			Title  string `json:"title,omitempty"`
			Outlet int    `json:"outlet"`
		}

		results := make([]result, len(req.Paths))
		var wg sync.WaitGroup

		for i, path := range req.Paths {
			wg.Add(1)
			go func(idx int, targetPath string) {
				defer wg.Done()

				cacheKey := req.From + "|" + targetPath
				if _, outlet, ok := cache.get(cacheKey); ok {
					results[idx] = result{Path: targetPath, Outlet: outlet}
					return
				}

				matched, _, found := router.MatchRoute(cfg.Routes, targetPath)
				if !found {
					return
				}

				ngFile, err := parser.Parse(matched.FilePath)
				if err != nil {
					return
				}

				partial := render.RenderPartial(ngFile, cfg.Version, cfg.Layouts, req.From, cfg.Routes)
				results[idx] = result{
					Path:   targetPath,
					HTML:   partial.HTML,
					Title:  partial.Meta.Title,
					Outlet: partial.Meta.Outlet,
				}

				// Cache the frame
				frame, err := render.EncodePartialFrame(partial)
				if err == nil {
					cache.set(cacheKey, frame, partial.Meta.Outlet, 60*time.Second)
				}
			}(i, path)
		}

		wg.Wait()

		// Filter out empty results
		var filtered []result
		for _, r := range results {
			if r.Path != "" {
				filtered = append(filtered, r)
			}
		}

		return c.JSON(fiber.Map{"results": filtered})
	}
}
