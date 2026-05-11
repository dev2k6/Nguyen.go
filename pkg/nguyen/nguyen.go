package nguyen

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dev2k6/Nguyen.go/internal/cache"
	"github.com/dev2k6/Nguyen.go/internal/config"
	"github.com/dev2k6/Nguyen.go/internal/geo"
	"github.com/dev2k6/Nguyen.go/internal/parser"
	"github.com/dev2k6/Nguyen.go/internal/render"
	"github.com/dev2k6/Nguyen.go/internal/router"
	"github.com/dev2k6/Nguyen.go/internal/server"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

type App struct {
	fiber     *fiber.App
	config    *config.NguyenConfig
	routes    []router.Route
	isrCache  *cache.ISR
	pagesDir  string
	publicDir string
	stylesDir string
	buildDir  string
	port      int
	host      string
	setup     func(*fiber.App)
}

func New(opts ...Option) *App {
	app := &App{
		pagesDir:  "pages",
		publicDir: "public",
		stylesDir: "styles",
		buildDir:  ".nguyen",
		port:      3000,
		host:      "0.0.0.0",
	}
	for _, opt := range opts {
		opt(app)
	}
	return app
}

func (a *App) Fiber() *fiber.App {
	return a.fiber
}

func (a *App) Listen(addr string) error {
	if addr != "" {
		parts := strings.Split(addr, ":")
		if len(parts) == 2 {
			a.host = parts[0]
			fmt.Sscanf(parts[1], "%d", &a.port)
		}
	}
	return a.serve()
}

func (a *App) serve() error {
	cfg, err := config.Load("config/nguyen.config.yml")
	if err != nil {
		cfg = config.DefaultConfig()
	}
	a.config = cfg

	if a.port > 0 {
		cfg.Server.Port = a.port
	}
	if a.host != "" {
		cfg.Server.Host = a.host
	}

	pagesAbs, _ := filepath.Abs(a.pagesDir)
	routes, err := router.Discover(pagesAbs)
	if err != nil {
		return fmt.Errorf("nguyen: route discovery failed: %w", err)
	}
	a.routes = routes

	a.fiber = fiber.New(fiber.Config{
		AppName:               cfg.Name,
		ServerHeader:          "Nguyen.go",
		DisableStartupMessage: true,
		ReadTimeout:           10 * time.Second,
		WriteTimeout:          30 * time.Second,
		IdleTimeout:           120 * time.Second,
		BodyLimit:             4 * 1024 * 1024,
	})

	a.fiber.Use(func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("  PANIC recovered: %v — %s %s", r, c.Method(), c.Path())
				err = c.Status(500).JSON(fiber.Map{"error": "Internal Server Error"})
			}
		}()
		return c.Next()
	})

	a.fiber.Use(func(c *fiber.Ctx) error {
		c.Set("X-Powered-By", fmt.Sprintf("Nguyen.go V%s", cfg.Version))
		c.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		return c.Next()
	})

	a.fiber.Use(compress.New(compress.Config{Level: compress.LevelBestSpeed}))
	a.fiber.Use(etag.New())
	a.fiber.Use(limiter.New(limiter.Config{
		Max:          600,
		Expiration:   1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string { return c.IP() },
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(429).JSON(fiber.Map{"error": "Too Many Requests"})
		},
	}))

	a.fiber.Get("/_nguyen/image", server.ImageHandler(cfg))
	a.fiber.Get("/_nguyen/runtime", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"version": cfg.Version})
	})

	a.fiber.Static("/styles", a.stylesDir)
	a.fiber.Static("/public", a.publicDir)

	if a.setup != nil {
		a.setup(a.fiber)
	}

	geoHandler := geo.NewHandler(&cfg.GEO, routes)
	a.fiber.Get("/sitemap.xml", geoHandler.Sitemap)
	a.fiber.Get("/robots.txt", geoHandler.Robots)
	if cfg.GEO.LLMTxt.Enabled {
		a.fiber.Get("/llms.txt", geoHandler.LLMTxt)
	}
	a.fiber.Get("/.well-known/ai-bot.txt", geoHandler.AIBot)

	a.isrCache = cache.NewISR(filepath.Join(a.buildDir, "cache/pages"))
	a.mountRoutes(pagesAbs)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	a.printBanner(addr)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		if err := a.fiber.Listen(addr); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-quit:
		fmt.Println()
		log.Println("  Shutting down gracefully...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.fiber.ShutdownWithContext(ctx); err != nil {
			return fmt.Errorf("nguyen: forced shutdown: %w", err)
		}
		log.Println("  Server stopped")
	case err := <-errCh:
		msg := err.Error()
		if strings.Contains(msg, "address already in use") ||
			strings.Contains(msg, "Only one usage of each socket address") {
			return fmt.Errorf("nguyen: port %d is already in use", cfg.Server.Port)
		}
		return fmt.Errorf("nguyen: %w", err)
	}
	return nil
}

func (a *App) mountRoutes(pagesAbs string) {
	cfg := a.config
	layouts, _ := router.FindLayouts(pagesAbs)

	for _, route := range a.routes {
		if route.Is404 {
			continue
		}
		r := route
		a.fiber.Get(r.Pattern, func(c *fiber.Ctx) error {
			return a.renderRoute(c, r, cfg, layouts)
		})
	}

	a.fiber.Use(func(c *fiber.Ctx) error {
		c.Status(404)
		for _, r := range a.routes {
			if r.Is404 {
				return a.renderRoute(c, r, cfg, layouts)
			}
		}
		return c.JSON(fiber.Map{"error": "Not Found", "path": c.Path()})
	})
}

func (a *App) renderRoute(c *fiber.Ctx, route router.Route, cfg *config.NguyenConfig, layouts []router.LayoutInfo) error {
	renderMode := cfg.Render.Mode

	if renderMode == "isr" {
		html, isStale, err := a.isrCache.Get(c.Path())
		if err == nil {
			if isStale {
				c.Set("Cache-Control", "s-maxage=60, stale-while-revalidate=3600")
				c.Set("X-Nguyen-ISR", "stale")
				go a.isrCache.BackgroundRevalidate(c.Path(), func(path string) (string, int, []string) {
					ngFile, parseErr := parser.Parse(route.FilePath)
					if parseErr != nil {
						return "", 0, nil
					}
					result := render.RenderSSR(ngFile, cfg.Version, layouts...)
					return render.InjectMetaTags(result.HTML, result), result.Revalidate, nil
				})
			} else {
				c.Set("Cache-Control", "s-maxage=3600, stale-while-revalidate=86400")
				c.Set("X-Nguyen-ISR", "fresh")
			}
			c.Set("Content-Type", "text/html; charset=utf-8")
			return c.SendString(html)
		}
	}

	ngFile, err := parser.Parse(route.FilePath)
	if err != nil {
		log.Printf("  Parse error for %s: %v", route.Pattern, err)
		return c.Status(500).SendString("Internal Server Error")
	}

	result := render.RenderSSR(ngFile, cfg.Version, layouts...)
	html := render.InjectMetaTags(result.HTML, result)

	if renderMode == "isr" && result.Revalidate > 0 {
		a.isrCache.Set(c.Path(), html, result.Revalidate)
		c.Set("Cache-Control", fmt.Sprintf("s-maxage=%d, stale-while-revalidate=86400", result.Revalidate))
		c.Set("X-Nguyen-ISR", "miss")
	}

	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.SendString(html)
}

func (a *App) printBanner(addr string) {
	fmt.Println()
	fmt.Printf("  ⬡ Nguyen.go — Production\n")
	fmt.Println()
	fmt.Printf("  ► http://%s\n", addr)
	fmt.Printf("  ► %d route(s) loaded\n", len(a.routes))
	fmt.Println()
}
