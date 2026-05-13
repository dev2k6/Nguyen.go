package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/dev2k6/Nguyen.go/internal/config"
	"github.com/dev2k6/Nguyen.go/internal/generator"
	"github.com/dev2k6/Nguyen.go/internal/geo"
	"github.com/dev2k6/Nguyen.go/internal/router"

	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export all pages to static HTML for CDN deployment",
	Long: `Pre-renders every .gox route into static HTML files.
The output is placed in .nguyen/static/ and can be served
by any static file server or CDN with zero Go runtime.

Example:
  nguyen export --pages pages --output .nguyen/static`,
	Run: func(cmd *cobra.Command, args []string) {
		runExport()
	},
}

var (
	flagExportOutput string
	flagExportPages  string
	flagExportConfig string
)

func init() {
	exportCmd.Flags().StringVarP(&flagExportOutput, "output", "o", ".nguyen/static", "Output directory")
	exportCmd.Flags().StringVar(&flagExportPages, "pages", "pages", "Pages directory")
	exportCmd.Flags().StringVarP(&flagExportConfig, "config", "c", "config/nguyen.config.yml", "Config file path")
}

func runExport() {
	cyan := "\033[36m"
	green := "\033[32m"
	reset := "\033[0m"

	// Validate pages directory
	if _, err := os.Stat(flagExportPages); os.IsNotExist(err) {
		log.Fatalf("  ✕ Pages directory not found: %s", flagExportPages)
	}

	// Check there are routes to export
	routes, err := router.Discover(flagExportPages)
	if err != nil {
		log.Fatalf("  ✕ Cannot discover routes: %v", err)
	}

	if len(routes) == 0 {
		log.Fatalf("  ✕ No routes found in %s", flagExportPages)
	}

	// Load config for GEO
	cfg, _ := config.Load(flagExportConfig)
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	fmt.Println()
	fmt.Printf("  %s⬡%s Nguyen.go — Static Export\n", cyan, reset)
	fmt.Println()

	gen := generator.New(generator.Options{
		PagesDir:  flagExportPages,
		OutputDir: flagExportOutput,
		Version:   cfg.Version,
	})

	if err := gen.Generate(); err != nil {
		fmt.Printf("\n  ✕ Export failed: %v\n", err)
		os.Exit(1)
	}

	// Generate and write sitemap.xml
	if cfg.GEO.Enabled && cfg.GEO.Sitemap.Enabled {
		sitemap := geo.GenerateSitemapXML(routes, &cfg.GEO)
		sitemapPath := filepath.Join(flagExportOutput, "sitemap.xml")
		os.WriteFile(sitemapPath, []byte(sitemap), 0644)
		fmt.Printf("  %s✓%s sitemap.xml generated\n", green, reset)
	}

	// Generate and write robots.txt
	if cfg.GEO.Enabled && cfg.GEO.Robots.Enabled {
		robots := geo.GenerateRobotsTxt(&cfg.GEO)
		robotsPath := filepath.Join(flagExportOutput, "robots.txt")
		os.WriteFile(robotsPath, []byte(robots), 0644)
		fmt.Printf("  %s✓%s robots.txt generated\n", green, reset)
	}

	// Generate and write llms.txt
	if cfg.GEO.Enabled && cfg.GEO.LLMTxt.Enabled {
		llm := geo.GenerateLLMtxt(routes, nil, &cfg.GEO)
		llmPath := filepath.Join(flagExportOutput, "llms.txt")
		os.WriteFile(llmPath, []byte(llm), 0644)
		fmt.Printf("  %s✓%s llms.txt generated\n", green, reset)
	}

	fmt.Println()
	fmt.Printf("  %s✓%s Export complete!\n", green, reset)
	fmt.Println()
	fmt.Printf("  Deploy %s to any static host or CDN.\n", flagExportOutput)
	fmt.Println()
}
