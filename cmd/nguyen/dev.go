package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dev2k6/Nguyen.go/internal/cache"
	"github.com/dev2k6/Nguyen.go/internal/config"
	"github.com/dev2k6/Nguyen.go/internal/parser"
	"github.com/dev2k6/Nguyen.go/internal/pwa"
	"github.com/dev2k6/Nguyen.go/internal/render"
	"github.com/dev2k6/Nguyen.go/internal/router"
	"github.com/dev2k6/Nguyen.go/internal/server"

	"github.com/gofiber/fiber/v2"
	"github.com/spf13/cobra"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Start the development server with Hot Module Replacement",
	Long: `Starts the Go development server in dev mode.
Watches .nguyen files for changes and automatically reloads
the browser via WebSocket for instant feedback.`,
	Run: func(cmd *cobra.Command, args []string) {
		runDev()
	},
}

var (
	flagPort     int
	flagPagesDir string
	flagConfig   string
)

func init() {
	devCmd.Flags().IntVarP(&flagPort, "port", "p", 3000, "Port to listen on")
	devCmd.Flags().StringVar(&flagPagesDir, "pages", "pages", "Pages directory")
	devCmd.Flags().StringVarP(&flagConfig, "config", "c", "config/nguyen.config.yml", "Config file path")
}

func runDev() {
	cyan := "\033[36m"
	reset := "\033[0m"

	// Resolve absolute paths
	pagesAbs, err := filepath.Abs(flagPagesDir)
	if err != nil {
		log.Fatalf("  ✕ Invalid pages directory: %v", err)
	}

	// Project root = parent of pages/ (so styles/, public/, app/ resolve from there)
	projectRoot := filepath.Dir(pagesAbs)
	publicDir := filepath.Join(projectRoot, "public")
	stylesDir := filepath.Join(projectRoot, "styles")
	appDir := filepath.Join(projectRoot, "app")

	app, err := server.New(server.Options{
		ConfigPath: flagConfig,
		PagesDir:   pagesAbs,
		PublicDir:  publicDir,
		StylesDir:  stylesDir,
		Port:       flagPort,
	})
	if err != nil {
		log.Fatalf("  ✕ Failed to create server: %v", err)
	}

	// Start HMR: file watcher + WebSocket
	watchDirs := []string{
		pagesAbs,
		appDir,
		stylesDir,
	}
	// Only watch dirs that exist
	var existingDirs []string
	for _, d := range watchDirs {
		if _, err := os.Stat(d); err == nil {
			existingDirs = append(existingDirs, d)
		}
	}
	if len(existingDirs) > 0 {
		server.SetupHMR(app.Fiber, existingDirs)
		fmt.Printf("  %s⬡%s HMR enabled — watching %d director(ies)\n", cyan, reset, len(existingDirs))
	}

	// Start Tailwind CSS watcher if bin/tailwindcss exists
	tailwindBin := findTailwindBin()
	if tailwindBin != "" {
		inputCSS := "styles/input.css"
		outputCSS := "styles/output.css"
		if _, err := os.Stat(inputCSS); err == nil {
			fmt.Printf("  %s⬡%s Starting Tailwind CSS watcher...\n", cyan, reset)
			go runTailwindWatch(tailwindBin, inputCSS, outputCSS)
		}
	}

	// Set up ISR cache with disk persistence
	isrCache := cache.NewISR(".nguyen/cache/pages")

	// Discover nested layouts
	layouts, _ := router.FindLayouts(pagesAbs)

	// Mount GEO routes (sitemap.xml, robots.txt, llms.txt)
	app.MountGEORoutes()

	// Serve PWA manifest and service worker in dev mode
	cfg, _ := config.Load(flagConfig)
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	if cfg.PWA.Enabled {
		app.Fiber.Get("/manifest.json", func(c *fiber.Ctx) error {
			m, err := pwa.GenerateManifest(cfg)
			if err != nil {
				return c.Status(404).SendString("PWA disabled")
			}
			return c.JSON(m)
		})
		app.Fiber.Get("/sw.js", func(c *fiber.Ctx) error {
			c.Set("Content-Type", "text/javascript; charset=utf-8")
			return c.SendString(pwa.GenerateServiceWorker(cfg, nil))
		})
	}

	// Mount SPA navigate endpoint (must be before route mounting)
	app.MountNavigateEndpoint(layouts)

	// Mount routes with SSR/ISR rendering
	app.MountRoutes(func(c *fiber.Ctx, route router.Route) error {
		renderMode := app.Config.Render.Mode

		// For ISR mode, check cache first with SWR
		if renderMode == "isr" {
			html, isStale, err := isrCache.Get(c.Path())
			if err == nil {
				if isStale {
					// Serve stale with SWR header, revalidate in background
					c.Set("Cache-Control", "s-maxage=0, stale-while-revalidate=3600")
					c.Set("X-Nguyen-ISR", "stale")
					go isrCache.BackgroundRevalidate(c.Path(), func(path string) (string, int, []string) {
						ngFile, parseErr := parser.Parse(route.FilePath)
						if parseErr != nil {
							return "", 0, nil
						}
						result := render.RenderSSR(ngFile, app.Config.Version, layouts...)
						return render.InjectMetaTags(result.HTML, result), result.Revalidate, nil
					})
				} else {
					c.Set("Cache-Control", "s-maxage=0, stale-while-revalidate=86400")
					c.Set("X-Nguyen-ISR", "fresh")
				}
				c.Set("Content-Type", "text/html; charset=utf-8")
				return c.SendString(html)
			}
		}

		// Parse .nguyen file
		ngFile, err := parser.Parse(route.FilePath)
		if err != nil {
			log.Printf("  ✕ Parse error for %s: %v\n", route.Pattern, err)
			return c.Status(500).SendString("Internal Server Error")
		}

		var html string

		switch renderMode {
		case "ssr", "isr":
			if app.Config.Render.Stream && renderMode == "ssr" {
				// Streaming SSR: chunked response
				return render.RenderSSRStream(c, ngFile, app.Config.Version, layouts...)
			}

			// Server-side rendering with variable interpolation + metadata
			result := render.RenderSSR(ngFile, app.Config.Version, layouts...)
			html = render.InjectMetaTags(result.HTML, result)

			// Inject PWA tags if enabled
			if cfg.PWA.Enabled {
				html = injectPWAToHTML(html)
			}

			// Cache for ISR
			if renderMode == "isr" && result.Revalidate > 0 {
				isrCache.Set(c.Path(), html, result.Revalidate)
				c.Set("Cache-Control", "s-maxage="+fmt.Sprint(result.Revalidate)+", stale-while-revalidate=86400")
				c.Set("X-Nguyen-ISR", "miss")
			}

		default:
			// CSR mode: return template as-is (WASM handles rendering)
			html = ngFile.HTMLTemplate
		}

		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(html)
	})

	fmt.Printf("  %s►%s Starting development server...\n", cyan, reset)
	fmt.Println()

	if err := app.Serve(); err != nil {
		log.Fatalf("  ✕ Server error: %v", err)
	}
}

// injectPWAToHTML inserts manifest link and service worker registration into HTML
func injectPWAToHTML(html string) string {
	if strings.Contains(html, "</head>") && !strings.Contains(html, "manifest.json") {
		manifestLink := `    <link rel="manifest" href="/manifest.json">` + "\n"
		html = strings.Replace(html, "</head>", manifestLink+"</head>", 1)
	}
	if strings.Contains(html, "</body>") {
		swReg := pwa.GenerateSWRegister()
		html = strings.Replace(html, "</body>", swReg+"\n</body>", 1)
	}
	return html
}

// findTailwindBin locates the tailwindcss binary in common locations.
func findTailwindBin() string {
	names := []string{"tailwindcss", "tailwindcss.exe"}
	dirs := []string{".", "bin", ".nguyen"}

	cwd, _ := os.Getwd()
	for _, dir := range dirs {
		for _, name := range names {
			path := filepath.Join(cwd, dir, name)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	return ""
}

// runTailwindWatch runs tailwindcss --watch in the background.
func runTailwindWatch(bin, input, output string) {
	yellow := "\033[33m"
	dim := "\033[2m"
	reset := "\033[0m"

	cmd := exec.Command(bin, "-i", input, "-o", output, "--watch")
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		fmt.Printf("  %s⚠%s Tailwind watcher failed: %v\n", yellow, reset, err)
		return
	}

	fmt.Printf("  %s%s Tailwind watching: %s → %s%s\n", dim, "⬡", input, output, reset)
}
