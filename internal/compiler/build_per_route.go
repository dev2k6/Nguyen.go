package compiler

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/dev2k6/Nguyen.go/internal/parser"
	"github.com/dev2k6/Nguyen.go/internal/router"
)

// BuildPerRouteResult holds the output of per-route compilation
type BuildPerRouteResult struct {
	Success    bool
	Output     string
	ChunksDir  string
	Manifest   map[string]string // route pattern → chunk filename
	SharedWasm string
	Errors     []string
}

// BuildPerRoute compiles each .gox page into its own .wasm file.
// Also builds a shared.wasm containing the common core code (DOM, Router).
func BuildPerRoute(pagesDir, outputDir string) (*BuildPerRouteResult, error) {
	tinygo, err := FindTinyGo()
	if err != nil {
		return nil, err
	}

	// Discover routes
	routes, err := router.Discover(pagesDir)
	if err != nil {
		return nil, fmt.Errorf("cannot discover routes: %w", err)
	}

	// Filter: skip internal files and 404
	var pageRoutes []router.Route
	for _, r := range routes {
		if r.Is404 {
			continue
		}
		if strings.Contains(r.FilePath, "_app") || strings.Contains(r.FilePath, "_layout") {
			continue
		}
		pageRoutes = append(pageRoutes, r)
	}

	if len(pageRoutes) == 0 {
		return nil, fmt.Errorf("no page routes found in %s", pagesDir)
	}

	// Create chunks directory
	chunksDir := filepath.Join(outputDir, "chunks")
	os.MkdirAll(chunksDir, 0755)

	// Temporary directory for generated Go files
	tmpDir := filepath.Join(outputDir, "tmp")
	os.MkdirAll(tmpDir, 0755)

	result := &BuildPerRouteResult{
		Success:   true,
		ChunksDir: chunksDir,
		Manifest:  make(map[string]string),
	}

// Step 1: Generate Go source for each page
	var pageGoFiles []string
	liveDir := filepath.Join(outputDir, "live")
	os.MkdirAll(liveDir, 0755)

	for _, route := range pageRoutes {
		ngFile, err := parser.Parse(route.FilePath)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: parse error: %v", route.Pattern, err))
			continue
		}

		// .live.gox files generate a livepage.Page implementation — skip TinyGo.
		if ngFile.IsLive {
			liveInfo := parser.TranspileLive(ngFile)
			goPath := filepath.Join(liveDir, liveInfo.PackageName+".go")
			if err := os.WriteFile(goPath, []byte(liveInfo.SourceCode), 0644); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: live write error: %v", route.Pattern, err))
			}
			// Live pages are not compiled to WASM — no manifest entry needed.
			continue
		}

		info := parser.Transpile(ngFile)
		goPath := filepath.Join(tmpDir, info.PackageName+".go")
		if err := os.WriteFile(goPath, []byte(info.SourceCode), 0644); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: write error: %v", route.Pattern, err))
			continue
		}
		pageGoFiles = append(pageGoFiles, goPath)

		// manifest: "/" → "index.wasm", "/about" → "about.wasm"
		chunkName := routePatternToChunk(route.Pattern)
		result.Manifest[route.Pattern] = chunkName
	}

	if len(result.Errors) > 0 {
		result.Success = false
		return result, fmt.Errorf("build errors:\n%s", strings.Join(result.Errors, "\n"))
	}

	// Step 2: Build each page as a separate .wasm concurrently.
	// TinyGo compilation is CPU-heavy; concurrency is bounded to GOMAXPROCS
	// to avoid overwhelming the system.
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	var errorCount int64
	var errorsMu sync.Mutex

	for i, goFile := range pageGoFiles {
		i := i
		goFile := goFile
		route := pageRoutes[i]
		chunkName := routePatternToChunk(route.Pattern)
		wasmPath := filepath.Join(chunksDir, chunkName)

		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			cmd := exec.Command(tinygo,
				"build",
				"-o", wasmPath,
				"-target", "wasm",
				"-opt=2",
				"-no-debug",
				goFile,
			)
			cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

			output, err := cmd.CombinedOutput()
			if err != nil {
				errorsMu.Lock()
				result.Errors = append(result.Errors,
					fmt.Sprintf("%s: tinygo build failed: %v\nOutput:\n%s", route.Pattern, err, string(output)))
				errorsMu.Unlock()
				atomic.AddInt64(&errorCount, 1)
				return
			}
			errorsMu.Lock()
			result.Output += string(output)
			errorsMu.Unlock()
		}()
	}

	wg.Wait()

	if errorCount > 0 {
		result.Success = false
		return result, fmt.Errorf("build errors:\n%s", strings.Join(result.Errors, "\n"))
	}

	// Step 3: Write manifest.json
	manifestJSON, err := json.MarshalIndent(result.Manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("cannot encode manifest: %w", err)
	}

	manifestPath := filepath.Join(chunksDir, "manifest.json")
	if err := os.WriteFile(manifestPath, manifestJSON, 0644); err != nil {
		return nil, fmt.Errorf("cannot write manifest: %w", err)
	}

	// Backward-compatible location for older bridge/runtime behavior
	legacyManifestPath := filepath.Join(outputDir, "manifest.json")
	_ = os.WriteFile(legacyManifestPath, manifestJSON, 0644)

	// Step 4: Clean up temporary files
	os.RemoveAll(tmpDir)

	return result, nil
}

// routePatternToChunk converts a route pattern to a chunk filename
func routePatternToChunk(pattern string) string {
	if pattern == "/" {
		return "index.wasm"
	}

	name := strings.TrimPrefix(pattern, "/")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "*", "all")
	name = strings.ReplaceAll(name, "[", "")
	name = strings.ReplaceAll(name, "]", "")
	name = strings.Trim(name, "._-")
	if name == "" {
		name = "route"
	}
	return name + ".wasm"
}
