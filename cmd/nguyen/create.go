package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create <project-name>",
	Short: "Scaffold a new Nguyen.go project",
	Long: `Creates a new Nguyen.go project from the built-in template.

The project includes:
  - pages/ with index.nguyen, about.nguyen, 404.nguyen
  - app/layout.nguyen root layout
  - styles/global.css design system
  - middleware/, api/, components/, lib/
  - Fiber app.go entry point
  - config/nguyen.config.yml

Use --tailwind to set up Tailwind CSS via the standalone CLI (no npm required).`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		projectName := args[0]
		if flagCreateName != "" {
			projectName = flagCreateName
		}
		runCreate(projectName)
	},
}

var (
	flagCreateName     string
	flagCreateTailwind bool
)

func init() {
	createCmd.Flags().StringVar(&flagCreateName, "name", "", "Project name (defaults to directory name)")
	createCmd.Flags().BoolVar(&flagCreateTailwind, "tailwind", false, "Set up Tailwind CSS via standalone CLI (no npm required)")
}

const tailwindVersion = "latest"

func runCreate(projectName string) {
	green := "\033[32m"
	cyan := "\033[36m"
	yellow := "\033[33m"
	dim := "\033[2m"
	bold := "\033[1m"
	reset := "\033[0m"

	if strings.Contains(projectName, " ") {
		fmt.Printf("  ✕ Project name must not contain spaces\n")
		os.Exit(1)
	}

	targetDir := projectName
	if !filepath.IsAbs(targetDir) {
		cwd, _ := os.Getwd()
		targetDir = filepath.Join(cwd, targetDir)
	}

	if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
		fmt.Printf("  ✕ Directory %s already exists\n", projectName)
		os.Exit(1)
	}

	templateDir := findTemplateDir()
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		fmt.Printf("  ✕ Template not found at %s\n", templateDir)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("%s  ⬡ Nguyen.go — Create Project%s\n", bold, reset)
	fmt.Println()

	fmt.Printf("  %s→%s Creating %s...\n", cyan, reset, projectName)
	fmt.Println()

	// Step 1: Download Tailwind standalone CLI if requested
	tailwindBin := ""
	if flagCreateTailwind {
		fmt.Printf("  %s↓%s Downloading Tailwind CSS standalone CLI...\n", cyan, reset)
		var err error
		tailwindBin, err = downloadTailwind(targetDir)
		if err != nil {
			fmt.Printf("  %s⚠%s Could not download Tailwind CLI: %v\n", yellow, reset, err)
			fmt.Printf("  %s⚠%s Falling back to CDN\n", yellow, reset)
			flagCreateTailwind = false
		}
		if tailwindBin != "" {
			fmt.Printf("  %s✓%s Tailwind CLI %s (%s/%s)\n", green, reset, "latest", runtime.GOOS, runtime.GOARCH)
			fmt.Println()
		}
	}

	// Step 2: Copy template files
	oldModule := "hello-nguyen"
	newModule := projectName

	copied := 0

	filepath.Walk(templateDir, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		base := filepath.Base(srcPath)
		if base == "node_modules" || base == ".git" || base == ".nguyen" || base == ".nguyen_build" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, _ := filepath.Rel(templateDir, srcPath)
		targetPath := filepath.Join(targetDir, relPath)

		if info.IsDir() {
			os.MkdirAll(targetPath, 0755)
			return nil
		}

		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}

		content := string(data)

		// Replace module name in .go and .mod files
		if strings.HasSuffix(srcPath, ".go") || strings.HasSuffix(srcPath, ".mod") {
			content = strings.ReplaceAll(content, oldModule, newModule)
		}

		// Update config name
		if strings.HasSuffix(srcPath, ".yml") {
			content = strings.ReplaceAll(content, "name: hello-nguyen", "name: "+newModule)
		}

		// Replace import paths in .nguyen files
		if strings.HasSuffix(srcPath, ".nguyen") {
			content = strings.ReplaceAll(content, oldModule, newModule)
		}

		// Tailwind: inject output.css link instead of CDN script
		if flagCreateTailwind && strings.HasSuffix(srcPath, "layout.nguyen") {
			content = injectTailwindLink(content)
		}

		os.MkdirAll(filepath.Dir(targetPath), 0755)
		if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
			return err
		}
		copied++

		fmt.Printf("  %s●%s %s\n", dim, reset, relPath)
		return nil
	})

	// Step 3: Tailwind setup — create input.css, config, and build output.css
	if flagCreateTailwind && tailwindBin != "" {
		fmt.Println()

		// Create styles/input.css
		inputCSS := `@import "tailwindcss";
`
		inputPath := filepath.Join(targetDir, "styles", "input.css")
		os.MkdirAll(filepath.Dir(inputPath), 0755)
		os.WriteFile(inputPath, []byte(inputCSS), 0644)
		fmt.Printf("  %s●%s styles/input.css\n", dim, reset)

		// Create tailwind.config.js (minimal — Tailwind v4 picks up content automatically)
		twConfig := `/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./pages/**/*.nguyen",
    "./app/**/*.nguyen",
    "./components/**/*.nguyen",
  ],
}
`
		twPath := filepath.Join(targetDir, "tailwind.config.js")
		os.WriteFile(twPath, []byte(twConfig), 0644)
		fmt.Printf("  %s●%s tailwind.config.js\n", dim, reset)

		// Build output.css
		fmt.Printf("\n  %s⚙%s Building Tailwind CSS...\n", cyan, reset)
		buildCmd := exec.Command(tailwindBin,
			"-i", filepath.Join(targetDir, "styles", "input.css"),
			"-o", filepath.Join(targetDir, "styles", "output.css"),
			"--minify",
		)
		buildCmd.Stdout = nil
		buildCmd.Stderr = nil
		if err := buildCmd.Run(); err != nil {
			fmt.Printf("  %s⚠%s Could not build CSS: %v\n", yellow, reset, err)
		} else {
			outputPath := filepath.Join(targetDir, "styles", "output.css")
			if info, err := os.Stat(outputPath); err == nil {
				fmt.Printf("  %s✓%s styles/output.css (%s)\n", green, reset, humanSize(info.Size()))
			}
		}
	}

	// Step 4: Summary
	fmt.Println()
	fmt.Printf("  %s✓%s Project %s created (%d files)\n", green, reset, bold+projectName+reset, copied)
	fmt.Println()
	fmt.Printf("  %sNext steps:%s\n", cyan, reset)
	fmt.Printf("    cd %s\n", projectName)
	if flagCreateTailwind {
		fmt.Printf("    %sEdit styles/input.css to customize your design%s\n", dim, reset)
	}
	fmt.Printf("    %snguyen dev%s\n", dim, reset)
	fmt.Println()
}

var tailwindHTTPClient = &http.Client{Timeout: 120 * time.Second}

// downloadTailwind downloads the Tailwind CSS standalone CLI binary for the current OS/arch.
// Returns the path to the binary, or an error.
func downloadTailwind(targetDir string) (string, error) {
	arch := runtime.GOARCH
	osName := runtime.GOOS

	// Map Go arch to Tailwind arch names
	twArch := arch
	switch arch {
	case "amd64":
		twArch = "x64"
	case "arm64":
		twArch = "arm64"
	default:
		return "", fmt.Errorf("unsupported architecture: %s", arch)
	}

	var ext string
	if osName == "windows" {
		ext = ".exe"
	}

	filename := fmt.Sprintf("tailwindcss-%s-%s%s", osName, twArch, ext)

	// Try multiple URLs (latest and versioned)
	urls := []string{
		fmt.Sprintf("https://github.com/tailwindlabs/tailwindcss/releases/latest/download/%s", filename),
	}

	binDir := filepath.Join(targetDir, "bin")
	os.MkdirAll(binDir, 0755)

	binName := "tailwindcss"
	if osName == "windows" {
		binName = "tailwindcss.exe"
	}
	binPath := filepath.Join(binDir, binName)

	// Download
	var resp *http.Response
	var err error
	for _, url := range urls {
		resp, err = tailwindHTTPClient.Get(url)
		if err == nil && resp.StatusCode == 200 {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
	if err != nil || resp == nil || resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to download Tailwind CLI (HTTP %v)", func() int {
			if resp != nil {
				return resp.StatusCode
			}
			return 0
		}())
	}
	defer resp.Body.Close()

	f, err := os.Create(binPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return "", err
	}

	// Make executable on Unix
	if osName != "windows" {
		os.Chmod(binPath, 0755)
	}

	return binPath, nil
}

// injectTailwindLink replaces any existing tailwind CDN scripts with a link to output.css
// findTemplateDir locates the examples/hello-nguyen template directory.
func findTemplateDir() string {
	cwd, _ := os.Getwd()
	candidate := filepath.Join(cwd, "examples", "hello-nguyen")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	candidate = filepath.Join(cwd, "..", "..", "examples", "hello-nguyen")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return candidate
}

func injectTailwindLink(content string) string {
	link := `    <link rel="stylesheet" href="/styles/output.css" />`

	// Remove existing tailwind CDN scripts if present
	lines := strings.Split(content, "\n")
	var cleaned []string
	for _, line := range lines {
		if strings.Contains(line, "cdn.tailwindcss.com") || strings.Contains(line, "tailwind.config.js") {
			continue
		}
		cleaned = append(cleaned, line)
	}
	content = strings.Join(cleaned, "\n")

	// Insert link before </head>
	if strings.Contains(content, "</head>") {
		return strings.Replace(content, "</head>", link+"\n</head>", 1)
	}

	return content
}

func humanSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
}
