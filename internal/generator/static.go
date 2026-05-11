package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/dev2k6/Nguyen.go/internal/parser"
	"github.com/dev2k6/Nguyen.go/internal/render"
	"github.com/dev2k6/Nguyen.go/internal/router"
)

// Options configures the static site generator
type Options struct {
	PagesDir  string
	OutputDir string
	Version   string
}

// StaticGenerator pre-renders all pages to static HTML files
type StaticGenerator struct {
	opts Options
}

// New creates a new static generator
func New(opts Options) *StaticGenerator {
	return &StaticGenerator{opts: opts}
}

// Generate walks all routes and pre-renders them to static HTML files concurrently.
// Output structure: .nguyen/static/index.html, .nguyen/static/about/index.html, etc.
// Concurrency is bounded to GOMAXPROCS to avoid overwhelming the CPU.
func (g *StaticGenerator) Generate() error {
	// Discover routes
	routes, err := router.Discover(g.opts.PagesDir)
	if err != nil {
		return fmt.Errorf("static: cannot discover routes: %w", err)
	}

	// Ensure output directory
	os.MkdirAll(g.opts.OutputDir, 0755)

	// Filter pages and deduplicate by normalized output path before rendering
	var pages []router.Route
	builtPaths := make(map[string]struct{})
	for _, r := range routes {
		if r.Is404 {
			continue
		}
		if strings.Contains(r.FilePath, "_app") || strings.Contains(r.FilePath, "_layout") {
			continue
		}
		outPath := filepath.ToSlash(routeToHTMLPath(g.opts.OutputDir, r.Pattern))
		if _, seen := builtPaths[outPath]; seen {
			// Two routes resolve to the same HTML file; skip the duplicate
			continue
		}
		builtPaths[outPath] = struct{}{}
		pages = append(pages, r)
	}

	if len(pages) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	var generated int64
	var errorCount int64
	var mu sync.Mutex
	var errs []string

	for _, route := range pages {
		route := route
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			ngFile, parseErr := parser.Parse(route.FilePath)
			if parseErr != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("%s: parse error: %v", route.Pattern, parseErr))
				mu.Unlock()
				atomic.AddInt64(&errorCount, 1)
				return
			}

			result := render.RenderSSR(ngFile, g.opts.Version)
			html := render.InjectMetaTags(result.HTML, result)

			outputPath := routeToHTMLPath(g.opts.OutputDir, route.Pattern)
			os.MkdirAll(filepath.Dir(outputPath), 0755)

			if writeErr := os.WriteFile(outputPath, []byte(html), 0644); writeErr != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("%s: write error: %v", route.Pattern, writeErr))
				mu.Unlock()
				atomic.AddInt64(&errorCount, 1)
				return
			}

			mu.Lock()
			fmt.Printf("  ✓ %-20s → %s (%s)\n", route.Pattern, outputPath, formatSize(len(result.HTML)))
			mu.Unlock()
			atomic.AddInt64(&generated, 1)
		}()
	}

	wg.Wait()

	fmt.Println()
	fmt.Printf("  Generated %d page(s) in %s\n", generated, g.opts.OutputDir)

	if errorCount > 0 {
		for _, e := range errs {
			fmt.Printf("  ✕ %s\n", e)
		}
		return fmt.Errorf("static: %d error(s) during generation", errorCount)
	}

	return nil
}

// routeToHTMLPath converts a route pattern to an output HTML file path
func routeToHTMLPath(outputDir, pattern string) string {
	// "/" → index.html
	// "/about" → about/index.html
	// "/blog/my-post" → blog/my-post/index.html
	if pattern == "/" {
		return filepath.Join(outputDir, "index.html")
	}

	clean := strings.TrimPrefix(pattern, "/")
	return filepath.Join(outputDir, clean, "index.html")
}

func formatSize(bytes int) string {
	switch {
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
