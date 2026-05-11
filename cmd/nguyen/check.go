package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dev2k6/Nguyen.go/internal/parser"

	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Validate .nguyen file syntax without building",
	Long: `Scans all .nguyen files in the pages/ directory and
validates their syntax. Checks for:
  - Proper frontmatter delimiters (---)
  - Valid Go code in frontmatter
  - Template syntax (interpolation, event bindings)
  - Dynamic route parameter naming`,
	Run: func(cmd *cobra.Command, args []string) {
		runCheck()
	},
}

var flagCheckPages string

func init() {
	checkCmd.Flags().StringVar(&flagCheckPages, "pages", "pages", "Pages directory to check")
}

func runCheck() {
	green := "\033[32m"
	yellow := "\033[33m"
	red := "\033[31m"
	dim := "\033[2m"
	bold := "\033[1m"
	reset := "\033[0m"

	fmt.Println()
	fmt.Printf("%s  ⬡ Nguyen.go — Syntax Check%s\n", bold, reset)
	fmt.Println()

	pagesDir := flagCheckPages
	if _, err := os.Stat(pagesDir); os.IsNotExist(err) {
		fmt.Printf("  %s✕%s Pages directory not found: %s\n", red, reset, pagesDir)
		fmt.Println()
		os.Exit(1)
	}

	// Discover .nguyen files
	var files []string
	filepath.Walk(pagesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".nguyen") {
			files = append(files, path)
		}
		return nil
	})

	if len(files) == 0 {
		fmt.Printf("  %s⚠%s  No .nguyen files found\n", yellow, reset)
		fmt.Println()
		return
	}

	// Check each file
	var (
		passed   int
		failed   int
		warnings int
		results  []checkResult
	)

	for _, f := range files {
		relPath, _ := filepath.Rel(pagesDir, f)
		result := checkFile(f, relPath)
		results = append(results, result)

		if result.err != nil {
			failed++
		} else {
			passed++
		}
		if len(result.warnings) > 0 {
			warnings += len(result.warnings)
		}
	}

	// Print results
	for _, r := range results {
		status := fmt.Sprintf("%s✓%s", green, reset)
		marker := fmt.Sprintf("  %s●%s", dim, reset)
		if r.err != nil {
			status = fmt.Sprintf("%s✕%s", red, reset)
			marker = fmt.Sprintf("  %s●%s", red, reset)
		}
		fmt.Printf("%s %-30s %s PASS\n", marker, r.path, status)

		if r.err != nil {
			fmt.Printf("    %sError:%s   %s\n", red, reset, r.err)
		}

		// Show component info
		if r.info != nil {
			fmt.Printf("    %sPackage:%s %s", dim, reset, r.info.PackageName)
			fmt.Printf("  %sStates:%s %d", dim, reset, len(r.info.StateVars))
			fmt.Printf("  %sEvents:%s %d\n", dim, reset, len(r.info.EventHandlers))
		}

		for _, w := range r.warnings {
			fmt.Printf("    %s⚠ Warning:%s %s\n", yellow, reset, w)
		}
	}

	// Summary
	fmt.Println()
	border := strings.Repeat("─", 40)
	fmt.Printf("  %s%s%s\n", dim, border, reset)
	fmt.Printf("  %sResults:%s  ", bold, reset)
	fmt.Printf("%s%d passed%s  ", green, passed, reset)
	if failed > 0 {
		fmt.Printf("%s%d failed%s  ", red, failed, reset)
	}
	if warnings > 0 {
		fmt.Printf("%s%d warnings%s", yellow, warnings, reset)
	}
	fmt.Println()
	fmt.Printf("  %s%s%s\n", dim, border, reset)
	fmt.Println()

	if failed > 0 {
		os.Exit(1)
	}
}

type checkResult struct {
	path     string
	err      error
	info     *parser.ComponentInfo
	warnings []string
}

func isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func checkFile(absPath, relPath string) checkResult {
	result := checkResult{path: relPath}

	// Parse file
	ngFile, err := parser.Parse(absPath)
	if err != nil {
		result.err = err
		return result
	}

	// Transpile
	info := parser.Transpile(ngFile)
	result.info = info

	// Generate warnings

	if len(info.EventHandlers) > 0 && ngFile.GoCode == "" {
		result.warnings = append(result.warnings,
			"Event handlers defined in template but no frontmatter Go code found")
	}

	// Check for common mistakes
	template := ngFile.HTMLTemplate
	openCount := strings.Count(template, "{")
	closeCount := strings.Count(template, "}")
	if openCount != closeCount {
		result.warnings = append(result.warnings,
			fmt.Sprintf("Mismatched curly braces: %d opening vs %d closing", openCount, closeCount))
	}

	if strings.Count(template, " @") > 0 {
		// Match @event= patterns only; mailto:foo@bar / http links / @media contain '@' but aren't events.
		tokens := strings.Split(template, " @")
		for i, tok := range tokens {
			if i == 0 {
				continue
			}
			// Is this an attribute binding? It must be followed by word chars then '='.
			end := 0
			for end < len(tok) && (isAlpha(tok[end]) || tok[end] == '-') {
				end++
			}
			name := tok[:end]
			if name == "" || end >= len(tok) || tok[end] != '=' {
				continue
			}
			known := map[string]bool{
				"click": true, "input": true, "submit": true, "change": true,
				"keydown": true, "keyup": true, "focus": true, "blur": true,
				"mouseenter": true, "mouseleave": true, "mouseover": true, "mouseout": true,
				"dblclick": true,
			}
			if !known[name] {
				result.warnings = append(result.warnings,
					"Unknown @"+name+" event binding — known: @click, @input, @submit, @change, @keydown, @keyup, @focus, @blur, @mouseenter, @mouseleave, @mouseover, @mouseout, @dblclick")
			}
		}
	}

	return result
}
