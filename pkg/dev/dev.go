// Package dev provides a one-call dev server for Nguyen.go applications.
// Import this package and call dev.Run() from your main() — the equivalent of
// `next dev` in Next.js.
package dev

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dev2k6/Nguyen.go/internal/cache"
	"github.com/dev2k6/Nguyen.go/internal/parser"
	"github.com/dev2k6/Nguyen.go/internal/render"
	"github.com/dev2k6/Nguyen.go/internal/router"
	"github.com/dev2k6/Nguyen.go/internal/server"

	"github.com/gofiber/fiber/v2"
)

// Config holds optional startup configuration for the dev server.
type Config struct {
	// Setup is an optional callback called after middleware is configured
	// but before routes are mounted. Use it to register custom API routes,
	// middleware, or static handlers on the Fiber app.
	Setup func(app *fiber.App)
}

// Run starts the Nguyen.go development server with SSR, HMR, and Tailwind watcher.
// It discovers .gox files from the pages/ directory, sets up file-system routing,
// and serves with the rendering mode specified in config.
//
// Usage in main():
//
//	package main
//	import (
//	    "hello-nguyen/api"
//	    "github.com/dev2k6/Nguyen.go/pkg/dev"
//	)
//	func main() { dev.Run(dev.Config{Setup: api.Register}) }
func Run(cfg ...Config) {
	cyan := "\033[36m"
	reset := "\033[0m"

	pagesDir := "pages"
	pagesAbs, _ := filepath.Abs(pagesDir)

	app, err := server.New(server.Options{
		ConfigPath: "config/nguyen.config.yml",
		PagesDir:   pagesAbs,
		PublicDir:  "public",
		Port:       3000,
	})
	if err != nil {
		log.Fatalf("  ✕ Failed to create server: %v", err)
	}

	// HMR: file watcher + WebSocket
	cwd, _ := os.Getwd()
	watchDirs := []string{pagesAbs, filepath.Join(cwd, "app"), filepath.Join(cwd, "styles")}
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

	// Tailwind CSS: run initial build, then watch for changes
	tailwindBin := findTailwindBin()
	inputCSS := "styles/input.css"
	outputCSS := "styles/output.css"
	var tailwindWatchCmd *exec.Cmd
	defer func() {
		if err := stopProcess(tailwindWatchCmd); err != nil {
			fmt.Printf("  ⚠ Tailwind watcher stop failed: %v\n", err)
		}
	}()
	if tailwindBin != "" && fileExists(inputCSS) {
		fmt.Printf("  %s⬡%s Building Tailwind CSS...\n", cyan, reset)
		if err := runTailwindBuild(tailwindBin, inputCSS, outputCSS); err != nil {
			fmt.Printf("  ⚠ Tailwind build failed: %v\n", err)
		} else {
			fmt.Printf("  %s⬡%s Tailwind watching: %s → %s\n", "\033[2m", reset, inputCSS, outputCSS)
			tailwindWatchCmd = runTailwindWatch(tailwindBin, inputCSS, outputCSS)
		}
	}
	if !fileExists(outputCSS) && fileExists(inputCSS) {
		// No Tailwind binary — generate a simple output.css from input.css
		// skipping Tailwind-specific directives (@import, @theme).
		if err := generateFallbackCSS(inputCSS, outputCSS); err == nil {
			fmt.Printf("  %s⬡%s Generated CSS fallback: %s\n", "\033[2m", reset, outputCSS)
		}
	}

	// Run user setup callback (API routes, custom middleware)
	if len(cfg) > 0 && cfg[0].Setup != nil {
		cfg[0].Setup(app.Fiber)
	}

	// ISR cache
	isrCache := cache.NewISR(".nguyen/cache/pages")

	// GEO routes
	app.MountGEORoutes()

	// File-system routes with SSR/ISR/CSR rendering
	app.MountRoutes(func(c *fiber.Ctx, route router.Route) error {
		renderMode := app.Config.Render.Mode

		if renderMode == "isr" {
			html, isStale, err := isrCache.Get(c.Path())
			if err == nil {
				if isStale {
					c.Set("Cache-Control", "s-maxage=60, stale-while-revalidate=3600")
					c.Set("X-Nguyen-ISR", "stale")
					go isrCache.BackgroundRevalidate(c.Path(), func(path string) (string, int, []string) {
						ngFile, parseErr := parser.Parse(route.FilePath)
						if parseErr != nil {
							return "", 0, nil
						}
						result := render.RenderSSR(ngFile, app.Config.Version)
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
			log.Printf("  ✕ Parse error for %s: %v\n", route.Pattern, err)
			return c.Status(500).SendString("Internal Server Error")
		}

		var html string

		switch renderMode {
		case "ssr", "isr":
			result := render.RenderSSR(ngFile, app.Config.Version)
			html = render.InjectMetaTags(result.HTML, result)

			if renderMode == "isr" && result.Revalidate > 0 {
				isrCache.Set(c.Path(), html, result.Revalidate)
				c.Set("Cache-Control", "s-maxage="+fmt.Sprint(result.Revalidate)+", stale-while-revalidate=86400")
				c.Set("X-Nguyen-ISR", "miss")
			}

		default:
			// CSR mode: compose layout + interpolate vars, skip metadata injection
			result := render.RenderSSR(ngFile, app.Config.Version)
			html = render.InjectMetaTags(result.HTML, result)
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

func findTailwindBin() string {
	names := []string{"tailwindcss", "tailwindcss.exe"}
	dirs := []string{".", "bin", ".nguyen"}
	cwd, _ := os.Getwd()
	for _, dir := range dirs {
		for _, name := range names {
			p := filepath.Join(cwd, dir, name)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func runTailwindBuild(bin, input, output string) error {
	cmd := exec.Command(bin, "-i", input, "-o", output, "--minify")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func runTailwindWatch(bin, input, output string) *exec.Cmd {
	dim := "\033[2m"
	reset := "\033[0m"
	cmd := exec.Command(bin, "-i", input, "-o", output, "--watch")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		fmt.Printf("  ⚠ Tailwind watcher failed: %v\n", err)
		return nil
	}
	fmt.Printf("  %s%s Tailwind watching: %s → %s%s\n", dim, "⬡", input, output, reset)
	return cmd
}

func stopProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	err := cmd.Process.Kill()
	if err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	_, waitErr := cmd.Process.Wait()
	if waitErr != nil && !strings.Contains(waitErr.Error(), "process already finished") {
		return waitErr
	}
	return nil
}

// generateFallbackCSS creates a minimal output.css from input.css by stripping
// Tailwind-specific directives (@import, @theme) and keeping plain CSS rules.
func generateFallbackCSS(input, output string) error {
	data, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	content := string(data)
	var sb strings.Builder
	for _, line := range splitLines(content) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "@import") || strings.HasPrefix(trimmed, "@theme") {
			continue
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return os.WriteFile(output, []byte(sb.String()), 0644)
}

func splitLines(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}
