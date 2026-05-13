package server

import (
	"context"
	_ "embed" // required for go:embed directive
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/dev2k6/Nguyen.go/internal/config"
	"github.com/dev2k6/Nguyen.go/internal/geo"
	"github.com/dev2k6/Nguyen.go/internal/router"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

//go:embed bridge.js
var bridgeJS []byte

// App wraps a Fiber application with Nguyen.go configuration
type App struct {
	Fiber      *fiber.App
	Config     *config.NguyenConfig
	Routes     []router.Route
	GeoHandler *geo.Handler
	ChunkMode  bool // true when per-route .wasm chunks + manifest exist
}

// Options configures the dev server
type Options struct {
	ConfigPath string // path to nguyen.config.yml
	PagesDir   string // path to pages/ directory
	PublicDir  string // path to public/ directory
	StylesDir  string // path to styles/ directory (empty = "./styles")
	Port       int    // override config port (0 = use config)
}

// New creates a new Nguyen.go server application
func New(opts Options) (*App, error) {
	// Load configuration
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("server: %w", err)
	}
	if opts.Port > 0 {
		cfg.Server.Port = opts.Port
	}

	// Discover routes
	routes, err := router.Discover(opts.PagesDir)
	if err != nil {
		return nil, fmt.Errorf("server: %w", err)
	}

	// Build Fiber app
	app := fiber.New(fiber.Config{
		AppName:               cfg.Name,
		ServerHeader:          "Nguyen.go",
		DisableStartupMessage: true,
		ReadTimeout:           5 * time.Second,
		WriteTimeout:          10 * time.Second,
		IdleTimeout:           30 * time.Second,
		BodyLimit:             4 * 1024 * 1024, // 4MB default — overridable via cfg
	})

	// Panic recovery — must be first middleware
	app.Use(func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("  ⚠ PANIC recovered: %v — %s %s\n", r, c.Method(), c.Path())
				err = c.Status(500).JSON(fiber.Map{
					"error": "Internal Server Error",
				})
			}
		}()
		return c.Next()
	})

	// Request logger — Next.js-style compact format
	green := "\033[32m"
	cyan := "\033[36m"
	dim := "\033[2m"
	red := "\033[31m"
	reset := "\033[0m"
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		duration := time.Since(start)
		status := c.Response().StatusCode()
		if status >= 500 {
			fmt.Printf("  %s%s%s %s%s%s %s %d %s%s%s (%s)\n", dim, c.Method(), reset, cyan, c.Path(), reset, cyan, status, red, http.StatusText(status), reset, duration.Round(100*time.Microsecond))
		} else if status >= 400 {
			fmt.Printf("  %s%s%s %s%s%s %s %d %s%s%s (%s)\n", dim, c.Method(), reset, cyan, c.Path(), reset, cyan, status, red, http.StatusText(status), reset, duration.Round(100*time.Microsecond))
		} else if status >= 300 {
			fmt.Printf("  %s%s%s %s%s%s %s %d %s%s%s (%s)\n", dim, c.Method(), reset, cyan, c.Path(), reset, cyan, status, green, "redirect", reset, duration.Round(100*time.Microsecond))
		} else {
			fmt.Printf("  %s%s%s %s%s%s %s %d %s%s%s (%s)\n", dim, c.Method(), reset, cyan, c.Path(), reset, cyan, status, green, http.StatusText(status), reset, duration.Round(100*time.Microsecond))
		}
		return err
	})

	// Set X-Powered-By header + baseline security headers for dev mode.
	// Production (`start.go`) adds HSTS on top; we omit HSTS here because dev
	// typically runs over plain HTTP on localhost.
	app.Use(func(c *fiber.Ctx) error {
		c.Set("X-Powered-By", fmt.Sprintf("Nguyen.go V%s", cfg.Version))
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		return c.Next()
	})

	// Middleware pipeline
	app.Use(compress.New(compress.Config{Level: compress.LevelBestSpeed}))
	app.Use(etag.New())
	app.Use(limiter.New(limiter.Config{
		Max:          600,
		Expiration:   1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string { return c.IP() },
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(429).JSON(fiber.Map{"error": "Too Many Requests"})
		},
	}))

	// Static files
	stylesPath := opts.StylesDir
	if stylesPath == "" {
		stylesPath = "./styles"
	}
	app.Static("/styles", stylesPath)
	app.Static("/public", opts.PublicDir)

	// Serve bridge.js from embedded file (always available)
	app.Get("/bridge/nguyen_bridge.js", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/javascript; charset=utf-8")
		return c.Send(bridgeJS)
	})

	// Built-in image optimization endpoint
	app.Get("/_nguyen/image", ImageHandler(cfg))

	// Runtime capability discovery (tells bridge.js if per-route chunks exist)
	// chunkMode is set after construction; use pointer capture
	chunkMode := false
	app.Get("/_nguyen/runtime", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"chunkMode": chunkMode,
			"version":   cfg.Version,
		})
	})

	return &App{
		Fiber:     app,
		Config:    cfg,
		Routes:    routes,
		ChunkMode: chunkMode,
	}, nil
}

// MountNavigateEndpoint registers the /_nguyen/navigate endpoint for SPA navigation
// and the /_nguyen/prefetch batch endpoint for concurrent multi-route prefetching.
func (a *App) MountNavigateEndpoint(layouts []router.LayoutInfo) {
	cfg := NavigateHandlerConfig{
		Routes:  a.Routes,
		Layouts: layouts,
		Version: a.Config.Version,
	}
	a.Fiber.Get("/_nguyen/navigate", NavigateHandler(cfg))
	a.Fiber.Post("/_nguyen/prefetch", BatchNavigateHandler(cfg))
}

// MountRoutes registers all file-system discovered routes on the Fiber app.
// Routes are mounted in priority order (static before dynamic).
// renderFunc receives the parsed .gox file for rendering.
func (a *App) MountRoutes(renderFunc func(*fiber.Ctx, router.Route) error) {
	for _, route := range a.Routes {
		if route.Is404 {
			// 404 is handled by catch-all middleware
			continue
		}
		r := route // capture
		a.Fiber.Get(r.Pattern, func(c *fiber.Ctx) error {
			return renderFunc(c, r)
		})
	}

	// 404 catch-all (must be last)
	a.Fiber.Use(func(c *fiber.Ctx) error {
		c.Status(404)
		for _, r := range a.Routes {
			if r.Is404 {
				return renderFunc(c, r)
			}
		}
		return c.JSON(fiber.Map{
			"error": "Not Found",
			"path":  c.Path(),
		})
	})
}

// MountGEORoutes registers GEO-specific routes (sitemap.xml, robots.txt, etc.)
func (a *App) MountGEORoutes() {
	a.GeoHandler = geo.NewHandler(&a.Config.GEO, a.Routes)

	a.Fiber.Get("/sitemap.xml", a.GeoHandler.Sitemap)
	a.Fiber.Get("/robots.txt", a.GeoHandler.Robots)
	if a.Config.GEO.LLMTxt.Enabled {
		a.Fiber.Get("/llms.txt", a.GeoHandler.LLMTxt)
	}
	a.Fiber.Get("/.well-known/ai-bot.txt", a.GeoHandler.AIBot)
}

// Serve starts the server and blocks until shutdown signal
func (a *App) Serve() error {
	cfg := a.Config
	addr := cfg.Server.Address()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// Print banner
	PrintBanner(cfg.Name, cfg.Version, addr, a.Routes)

	errCh := make(chan error, 1)
	go func() {
		if err := a.Fiber.Listen(addr); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-quit:
		// graceful shutdown below
	case err := <-errCh:
		msg := err.Error()
		if strings.Contains(msg, "address already in use") ||
			strings.Contains(msg, "Only one usage of each socket address") ||
			strings.Contains(msg, "bind: permission denied") {
			return fmt.Errorf("server: %s is already in use; pass --port <n> to pick another port", addr)
		}
		return fmt.Errorf("server: listen failed: %w", err)
	}

	// Graceful shutdown
	fmt.Println()
	cyan := "\033[36m"
	reset := "\033[0m"
	log.Printf("  %s⬡%s Shutting down gracefully...\n", cyan, reset)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.Fiber.ShutdownWithContext(ctx); err != nil {
		return fmt.Errorf("server: forced shutdown: %w", err)
	}
	log.Printf("  %s✓%s Server stopped\n", cyan, reset)
	return nil
}

// PrintBanner displays the startup banner with routes and configuration.
func PrintBanner(name, version, addr string, routes []router.Route) {
	cyan := "\033[36m"
	green := "\033[32m"
	yellow := "\033[33m"
	dim := "\033[2m"
	reset := "\033[0m"
	bold := "\033[1m"

	fmt.Println()
	fmt.Printf("%s  ⬡ %s %s%s\n", bold, name, cyan, version)
	fmt.Println()
	fmt.Printf("  %s►%s Ready in %s%.1f%s\n", cyan, reset, green, 0.9+float64(len(routes))*0.3, "s")
	fmt.Println()
	fmt.Printf("  %sLocal:%s    http://%s\n", cyan, reset, addr)
	fmt.Println()

	// Route table
	fmt.Printf("  %sRoutes (%d):%s\n", cyan, len(routes), reset)
	for _, r := range routes {
		marker := green + "●" + reset
		if r.Is404 {
			marker = yellow + "○" + reset
		}
		dynamic := ""
		for _, seg := range r.Segments {
			if seg.Type == router.Dynamic {
				dynamic = " [dynamic]"
			} else if seg.Type == router.CatchAll {
				dynamic = " [catch-all]"
			}
		}
		fmt.Printf("    %s %-18s → %s%s\n", marker, r.Pattern, fileBase(r.FilePath), dynamic)
	}
	fmt.Println()

	fmt.Printf("  %sPress %sCtrl+C%s to stop%s\n", yellow, reset, dim, reset)
	fmt.Println()
}

func fileBase(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}
