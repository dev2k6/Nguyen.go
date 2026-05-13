package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dev2k6/Nguyen.go/internal/compiler"
	"github.com/dev2k6/Nguyen.go/internal/config"
	"github.com/dev2k6/Nguyen.go/internal/parser"
	"github.com/dev2k6/Nguyen.go/internal/pwa"
	"github.com/dev2k6/Nguyen.go/internal/render"
	"github.com/dev2k6/Nguyen.go/internal/router"

	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build your .gox files into optimized WebAssembly",
	Long: `Compiles all .gox files in the pages/ directory into
a single app.wasm binary using TinyGo. The output is
optimized for production deployment with minimal bundle size.`,
	Run: func(cmd *cobra.Command, args []string) {
		runBuild()
	},
}

var (
	flagBuildOutput   string
	flagBuildPages    string
	flagBuildOptimize bool
	flagPerRoute      bool
	flagAnalyze       bool
	flagNoWASM        bool
)

func init() {
	buildCmd.Flags().StringVarP(&flagBuildOutput, "output", "o", ".nguyen", "Output directory")
	buildCmd.Flags().StringVar(&flagBuildPages, "pages", "pages", "Pages directory")
	buildCmd.Flags().BoolVar(&flagBuildOptimize, "optimize", true, "Enable TinyGo optimizations")
	buildCmd.Flags().BoolVar(&flagPerRoute, "per-route", true, "Code-split per route (separate .wasm per page)")
	buildCmd.Flags().BoolVar(&flagAnalyze, "analyze", false, "Generate bundle size report")
	buildCmd.Flags().BoolVar(&flagNoWASM, "no-wasm", false, "Skip TinyGo WASM compilation (SSR-only production build)")
}

func runBuild() {
	cyan := "\033[36m"
	green := "\033[32m"
	yellow := "\033[33m"
	dim := "\033[2m"
	reset := "\033[0m"

	start := time.Now()

	fmt.Println()
	fmt.Println("  ⬡ Nguyen.go")
	fmt.Println()
	fmt.Printf("  %s✓%s Building project...\n", cyan, reset)
	fmt.Println()

	pagesDir := flagBuildPages
	if _, err := os.Stat(pagesDir); os.IsNotExist(err) {
		log.Fatalf("  %s✕%s Pages directory not found: %s\n", yellow, reset, pagesDir)
	}

	var nguyenFiles []string
	filepath.Walk(pagesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".gox") {
			nguyenFiles = append(nguyenFiles, path)
		}
		return nil
	})

	if len(nguyenFiles) == 0 {
		log.Fatalf("  %s✕%s No .gox files found in %s\n", yellow, reset, pagesDir)
	}

	fmt.Printf("  %sFound %d .gox file(s)%s\n", dim, len(nguyenFiles), reset)
	fmt.Println()

	outputDir := flagBuildOutput
	os.MkdirAll(outputDir, 0755)

	var componentInfos []*parser.ComponentInfo
	parseErrors := 0

	for _, f := range nguyenFiles {
		relPath, _ := filepath.Rel(pagesDir, f)
		fmt.Printf("  %s●%s %s", dim, reset, relPath)

		ngFile, err := parser.Parse(f)
		if err != nil {
			fmt.Printf("  %s✕ PARSE ERROR%s\n    %v\n", yellow, reset, err)
			parseErrors++
			continue
		}

		info := parser.Transpile(ngFile)
		componentInfos = append(componentInfos, info)

		outPath := filepath.Join(outputDir, info.PackageName+".go")
		if err := os.WriteFile(outPath, []byte(info.SourceCode), 0644); err != nil {
			fmt.Printf("  %s✕ WRITE ERROR%s\n    %v\n", yellow, reset, err)
			parseErrors++
			continue
		}

		fmt.Printf("  %s✓%s %s (%d state, %d events)\n",
			green, reset, outPath, len(info.StateVars), len(info.EventHandlers))
	}

	if parseErrors > 0 {
		fmt.Println()
		fmt.Printf("  %s✕ Build failed with %d error(s)%s\n", yellow, parseErrors, reset)
		fmt.Println()
		os.Exit(1)
	}

	elapsed := time.Since(start)

	fmt.Println()
	fmt.Printf("  %s✓%s Compiled %d component(s) successfully\n", green, reset, len(componentInfos))
	fmt.Println()
	fmt.Printf("  %sOutput:%s  %s\n", cyan, reset, outputDir)
	fmt.Printf("  %sTime:%s    %s%.2fs%s\n", cyan, reset, dim, elapsed.Seconds(), reset)
	fmt.Println()

	var analyzeReport *compiler.SizeReport
	wasmBuildSucceeded := true
	if flagNoWASM {
		wasmBuildSucceeded = false
		fmt.Printf("  %s→%s WASM compilation skipped (--no-wasm)\n", dim, reset)
	} else if flagPerRoute {
		fmt.Printf("  %s⬡%s Compiling with per-route code splitting...\n", cyan, reset)
		fmt.Println()

		perRouteResult, err := compiler.BuildPerRoute(pagesDir, outputDir)
		if err != nil {
			wasmBuildSucceeded = false
			fmt.Printf("  %s⚠%s Per-route build failed:\n", yellow, reset)
			fmt.Printf("    %s%s\n", dim, err.Error())
			if perRouteResult != nil && len(perRouteResult.Errors) > 0 {
				for _, e := range perRouteResult.Errors {
					fmt.Printf("    %s✕%s %s\n", yellow, reset, e)
				}
			}
		} else {
			fmt.Printf("  %s✓%s Code-split build complete\n", green, reset)
			fmt.Printf("  %sChunks:%s %s/ (%d routes)\n", cyan, reset, perRouteResult.ChunksDir, len(perRouteResult.Manifest))
			for pattern, chunk := range perRouteResult.Manifest {
				chunkPath := filepath.Join(perRouteResult.ChunksDir, chunk)
				fmt.Printf("    %s%-20s%s → %s (%s)\n", dim, pattern, reset, chunk, formatSize(chunkPath))
			}
			if flagAnalyze {
				if report, reportErr := compiler.AnalyzeMulti(perRouteResult.ChunksDir); reportErr != nil {
					fmt.Printf("  %s⚠%s Bundle analyze failed: %v\n", yellow, reset, reportErr)
				} else {
					analyzeReport = report
				}
			}
		}
	} else {
		fmt.Printf("  %s⬡%s Compiling to WebAssembly via TinyGo...\n", cyan, reset)
		fmt.Println()

		result, err := compiler.BuildWASM(outputDir, outputDir)
		if err != nil {
			wasmBuildSucceeded = false
			fmt.Printf("  %s⚠%s TinyGo not available or build failed:\n", yellow, reset)
			fmt.Printf("    %s%s\n", dim, err.Error())
			fmt.Println()
			if result != nil && result.Output != "" {
				fmt.Printf("  %sOutput:%s\n%s\n", cyan, reset, result.Output)
			}
			fmt.Printf("  %s→%s Install TinyGo from https://tinygo.org/getting-started/install/\n", dim, reset)
			fmt.Println()
			fmt.Printf("  %s→%s Then run: %stinygo build -o %s/app.wasm -target wasm %s/*.go%s\n",
				dim, reset, dim, outputDir, outputDir, reset)
		} else {
			fmt.Printf("  %s✓%s WebAssembly compiled successfully\n", green, reset)
			fmt.Printf("  %sOutput:%s %s (%s)\n", cyan, reset, result.WasmPath,
				formatSize(result.WasmPath))
			if flagAnalyze {
				if report, reportErr := compiler.AnalyzeWASM(result.WasmPath); reportErr != nil {
					fmt.Printf("  %s⚠%s Bundle analyze failed: %v\n", yellow, reset, reportErr)
				} else {
					analyzeReport = report
				}
			}
		}
	}

	if !wasmBuildSucceeded && !flagNoWASM {
		fmt.Println()
		fmt.Printf("  %s⚠%s WASM artifacts were not generated — continuing with SSR/static HTML only\n", yellow, reset)
		fmt.Printf("  %s→%s Hydration will be disabled until TinyGo is fixed or %s--no-wasm%s is removed\n", dim, reset, dim, reset)
		fmt.Println()
	}

	if flagAnalyze && analyzeReport != nil {
		reportPath := filepath.Join(outputDir, "bundle-report.html")
		reportHTML := compiler.BuildReportHTML(analyzeReport)
		if err := os.WriteFile(reportPath, []byte(reportHTML), 0644); err != nil {
			fmt.Printf("  %s⚠%s Cannot write bundle report: %v\n", yellow, reset, err)
		} else {
			fmt.Printf("  %s✓%s Bundle report generated: %s\n", green, reset, reportPath)
		}
	} else if flagAnalyze {
		fmt.Printf("  %s⚠%s Bundle report skipped (no successful wasm output)\n", yellow, reset)
	}

	fmt.Println()
	if err := compiler.CopyBridge(outputDir); err != nil {
		fmt.Printf("  %s⚠%s Bridge JS not copied: %v\n", yellow, reset, err)
	} else {
		fmt.Printf("  %s✓%s Bridge JS copied\n", green, reset)
	}

	// Generate PWA assets (manifest + service worker)
	fmt.Println()
	fmt.Printf("  %s⬡%s Generating PWA assets...\n", cyan, reset)
	cfg, _ := config.Load(flagConfig)
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	version := cfg.Version

	if cfg.PWA.Enabled {
		var assets []string
		if flagPerRoute {
			chunksDir := filepath.Join(outputDir, "chunks")
			entries, _ := os.ReadDir(chunksDir)
			for _, e := range entries {
				if !e.IsDir() && (strings.HasSuffix(e.Name(), ".wasm") || e.Name() == "manifest.json") {
					assets = append(assets, "/chunks/"+e.Name())
				}
			}
		}
		bridgePath := filepath.Join(outputDir, "bridge.js")
		if _, err := os.Stat(bridgePath); err == nil {
			assets = append(assets, "/bridge.js")
		}
		if err := pwa.WriteAll(cfg, outputDir, assets); err != nil {
			fmt.Printf("  %s⚠%s PWA generation failed: %v\n", yellow, reset, err)
		} else {
			fmt.Printf("  %s✓%s manifest.json + sw.js generated\n", green, reset)
		}
	} else {
		fmt.Printf("  %s%s PWA disabled in config%s\n", dim, "→", reset)
	}

	// Generate static HTML files for production server
	fmt.Println()
	fmt.Printf("  %s⬡%s Generating static HTML...\n", cyan, reset)

	routes, err := router.Discover(pagesDir)
	if err == nil {
		generated := 0
		for _, route := range routes {
			if route.Is404 {
				continue
			}
			if strings.Contains(route.FilePath, "_app") || strings.Contains(route.FilePath, "_layout") {
				continue
			}
			// Skip dynamic routes (can't generate static HTML without params)
			hasDynamic := false
			for _, seg := range route.Segments {
				if seg.Type == router.Dynamic || seg.Type == router.CatchAll || seg.Type == router.Optional {
					hasDynamic = true
					break
				}
			}
			if hasDynamic {
				continue
			}

			ngFile, parseErr := parser.Parse(route.FilePath)
			if parseErr != nil {
				fmt.Printf("    %s⚠%s %s parse error: %v\n", yellow, reset, route.Pattern, parseErr)
				continue
			}

			layouts, _ := router.FindLayouts(pagesDir)
			result := render.RenderSSR(ngFile, version, layouts...)
			html := wrapHTMLShell(result, cfg)

			var htmlPath string
			if route.Pattern == "/" {
				htmlPath = filepath.Join(outputDir, "index.html")
			} else {
				htmlPath = filepath.Join(outputDir, strings.TrimPrefix(route.Pattern, "/"), "index.html")
			}
			os.MkdirAll(filepath.Dir(htmlPath), 0755)
			if writeErr := os.WriteFile(htmlPath, []byte(html), 0644); writeErr != nil {
				fmt.Printf("    %s⚠%s %s write error: %v\n", yellow, reset, route.Pattern, writeErr)
			} else {
				fmt.Printf("    %s✓%s %s → %s\n", green, reset, route.Pattern, htmlPath)
				generated++
			}
		}
		fmt.Printf("  %s✓%s Generated %d static HTML file(s)\n", green, reset, generated)
	}

	// Copy static asset dirs into the build output so `start` can serve them
	// standalone (independent of CWD).
	fmt.Println()
	fmt.Printf("  %s⬡%s Copying static assets...\n", cyan, reset)
	for _, dir := range []string{"styles", "public"} {
		src := dir
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(outputDir, dir)
		if err := copyDir(src, dst); err != nil {
			fmt.Printf("    %s⚠%s %s: %v\n", yellow, reset, dir, err)
		} else {
			fmt.Printf("    %s✓%s %s → %s\n", green, reset, dir, dst)
		}
	}

	fmt.Println()
}

// copyDir recursively copies src directory tree into dst.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return relErr
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

// wrapHTMLShell wraps an SSR result in the standard HTML shell.
// If the result already contains a full HTML document (layout mode),
// meta tags are injected into the existing <head> instead of nesting.
func wrapHTMLShell(result *render.Result, cfg *config.NguyenConfig) string {
	// Layout mode: result.HTML already has <html>/<head>/<body>
	if strings.Contains(result.HTML, "<html") {
		html := render.InjectMetaTags(result.HTML, result)
		return injectPWA(html, cfg)
	}
	// No layout: wrap with shell
	html := render.WrapHTML(result.HTML, result)
	return injectPWA(html, cfg)
}

// injectPWA inserts manifest link and service worker registration into HTML
// if PWA is enabled in the configuration.
func injectPWA(html string, cfg *config.NguyenConfig) string {
	if cfg == nil || !cfg.PWA.Enabled {
		return html
	}

	// Inject manifest link into <head> if not already present
	if strings.Contains(html, "</head>") && !strings.Contains(html, "manifest.json") {
		manifestLink := `    <link rel="manifest" href="/manifest.json">` + "\n"
		html = strings.Replace(html, "</head>", manifestLink+"</head>", 1)
	}

	// Inject service worker registration before </body>
	if strings.Contains(html, "</body>") {
		swReg := pwa.GenerateSWRegister()
		html = strings.Replace(html, "</body>", swReg+"\n</body>", 1)
	}

	return html
}

func formatSize(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "unknown"
	}
	size := info.Size()
	switch {
	case size >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	case size >= 1024:
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	default:
		return fmt.Sprintf("%d B", size)
	}
}
