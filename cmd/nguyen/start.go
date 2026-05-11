package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dev2k6/Nguyen.go/internal/config"
	ngserver "github.com/dev2k6/Nguyen.go/internal/server"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the production server from build output",
	Long: `Starts a production server serving the built .nguyen/ directory.

Static files, app.wasm, and HTML skeletons are served with
compression, caching headers, and optimized Fiber settings.`,
	Run: func(cmd *cobra.Command, args []string) {
		runStart()
	},
}

var (
	flagStartPort   int
	flagStartDir    string
	flagStartConfig string
)

func init() {
	startCmd.Flags().IntVarP(&flagStartPort, "port", "p", 3000, "Port to listen on")
	startCmd.Flags().StringVarP(&flagStartDir, "dir", "d", ".nguyen", "Build output directory")
	startCmd.Flags().StringVarP(&flagStartConfig, "config", "c", "config/nguyen.config.yml", "Config file path")
}

func runStart() {
	cyan := "\033[36m"
	green := "\033[32m"
	yellow := "\033[33m"
	dim := "\033[2m"
	bold := "\033[1m"
	reset := "\033[0m"

	buildDir := flagStartDir
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		log.Fatalf("  ✕ Build directory not found: %s\n    Run 'nguyen build' first.", buildDir)
	}

	cfg, err := config.Load(flagStartConfig)
	if err != nil {
		log.Printf("  ⚠ Failed to load config, using defaults: %v", err)
		cfg = config.DefaultConfig()
	}

	app := fiber.New(fiber.Config{
		AppName:               "Nguyen.go",
		ServerHeader:          "Nguyen.go",
		BodyLimit:             4 * 1024 * 1024,
		DisableStartupMessage: true,
	})

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

	app.Use(func(c *fiber.Ctx) error {
		c.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		return c.Next()
	})

	app.Use(compress.New(compress.Config{Level: compress.LevelBestSpeed}))
	app.Use(etag.New())
	// Baseline request-rate protection. 600 req/min/IP covers legitimate
	// page browsing well while limiting brute-force and crawler floods.
	app.Use(limiter.New(limiter.Config{
		Max:          600,
		Expiration:   1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string { return c.IP() },
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(429).JSON(fiber.Map{"error": "Too Many Requests"})
		},
	}))

	app.Get("/_nguyen/image", ngserver.ImageHandler(cfg))

	// Runtime capability discovery — detect if per-route chunks exist
	chunkMode := false
	manifestPath := filepath.Join(buildDir, "chunks", "manifest.json")
	if _, err := os.Stat(manifestPath); err == nil {
		chunkMode = true
	}
	app.Get("/_nguyen/runtime", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"chunkMode": chunkMode,
			"version":   cfg.Version,
		})
	})

	app.Static("/chunks", filepath.Join(buildDir, "chunks"))
	// Prefer assets copied into the build dir; fall back to CWD-local dirs.
	stylesDir := filepath.Join(buildDir, "styles")
	if _, err := os.Stat(stylesDir); err != nil {
		stylesDir = "styles"
	}
	publicDir := filepath.Join(buildDir, "public")
	if _, err := os.Stat(publicDir); err != nil {
		publicDir = "public"
	}
	app.Static("/styles", stylesDir)
	app.Static("/public", publicDir)

	app.Get("/bridge/nguyen_bridge.js", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/javascript; charset=utf-8")
		bridgePath := filepath.Join(buildDir, "bridge.js")
		if _, err := os.Stat(bridgePath); err == nil {
			return c.SendFile(bridgePath)
		}
		return c.SendStatus(404)
	})

	app.Get("/bridge.js", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/javascript; charset=utf-8")
		return c.SendFile(filepath.Join(buildDir, "bridge.js"))
	})

	app.Get("/app.wasm", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "application/wasm")
		c.Set("Cache-Control", "public, max-age=31536000, immutable")
		return c.SendFile(filepath.Join(buildDir, "app.wasm"))
	})

	app.Static("/", buildDir)

	absBuildDir, absErr := filepath.Abs(buildDir)
	if absErr != nil {
		absBuildDir = buildDir
	}
	indexPath := filepath.Join(buildDir, "index.html")
	notFoundPath := filepath.Join(buildDir, "404.html")
	app.Use(func(c *fiber.Ctx) error {
		// Try route-specific index.html first (built by SSR static generation)
		if c.Method() == fiber.MethodGet {
			trimmed := strings.Trim(c.Path(), "/")
			if trimmed != "" {
				candidate := filepath.Join(buildDir, trimmed, "index.html")
				// Defense-in-depth: ensure the resolved path stays inside the
				// build directory after symlink resolution. Blocks traversal
				// via URL-decoded "../" segments.
				if absCandidate, err := filepath.Abs(candidate); err == nil {
					if strings.HasPrefix(absCandidate, absBuildDir+string(filepath.Separator)) || absCandidate == absBuildDir {
						if _, statErr := os.Stat(absCandidate); statErr == nil {
							c.Set("Content-Type", "text/html; charset=utf-8")
							return c.SendFile(absCandidate)
						}
					}
				}
			}
		}
		// Try a dedicated 404.html
		if _, err := os.Stat(notFoundPath); err == nil {
			c.Set("Content-Type", "text/html; charset=utf-8")
			return c.Status(404).SendFile(notFoundPath)
		}
		// Fall back to root index.html (SPA-style)
		if _, err := os.Stat(indexPath); err == nil {
			c.Set("Content-Type", "text/html; charset=utf-8")
			return c.SendFile(indexPath)
		}
		return c.Status(404).JSON(fiber.Map{
			"error": "Not Found",
			"path":  c.Path(),
		})
	})

	fmt.Println()
	fmt.Printf("%s  ⬡ Nguyen.go%s — Production Server\n", bold, reset)
	fmt.Println()
	fmt.Printf("  %s►%s Starting production server...\n", cyan, reset)
	fmt.Println()
	fmt.Printf("  %sLocal:%s    http://localhost:%d\n", cyan, reset, flagStartPort)
	fmt.Printf("  %sBuild:%s    %s\n", cyan, reset, buildDir)
	fmt.Println()
	fmt.Printf("  %sPress %sCtrl+C%s to stop%s\n", yellow, reset, dim, reset)
	fmt.Println()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		addr := fmt.Sprintf(":%d", flagStartPort)
		if err := app.Listen(addr); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-quit:
		fmt.Println()
		log.Printf("  %s⬡%s Shutting down gracefully...\n", cyan, reset)
		if err := app.Shutdown(); err != nil {
			log.Printf("  ✕ Shutdown error: %v", err)
		}
		log.Printf("  %s✓%s Server stopped\n", green, reset)
	case err := <-errCh:
		// Clear, actionable listen failure messaging.
		msg := err.Error()
		if strings.Contains(msg, "address already in use") ||
			strings.Contains(msg, "Only one usage of each socket address") ||
			strings.Contains(msg, "bind: permission denied") {
			log.Fatalf("  ✕ Port %d is already in use. Start with --port <n> to pick another port.", flagStartPort)
		}
		log.Fatalf("  ✕ Failed to start server: %v", err)
	}
}
