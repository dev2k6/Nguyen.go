package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dev2k6/Nguyen.go/internal/version"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose the local toolchain and project layout",
	Long: `Doctor runs a series of checks against the host environment and
the current project. It reports Go version, TinyGo availability,
Tailwind binary, project layout (pages/, app/, config/, public/),
config validity, and obvious wiring issues so issues are caught
before "nguyen dev" or "nguyen build".`,
	Run: func(cmd *cobra.Command, args []string) {
		runDoctor()
	},
}

func init() {
	RootCmd.AddCommand(doctorCmd)
}

const (
	colReset  = "\033[0m"
	colDim    = "\033[2m"
	colBold   = "\033[1m"
	colRed    = "\033[31m"
	colGreen  = "\033[32m"
	colYellow = "\033[33m"
	colCyan   = "\033[36m"
)

type checkStatus int

const (
	statusOK checkStatus = iota
	statusWarn
	statusFail
	statusInfo
)

type doctorResult struct {
	label  string
	detail string
	status checkStatus
	hint   string
}

func runDoctor() {
	fmt.Println()
	fmt.Printf("%s  ⬡ Nguyen.go — Doctor%s\n", colBold, colReset)
	fmt.Printf("%s  Framework v%s%s\n", colDim, version.Version, colReset)
	fmt.Println()

	results := []doctorResult{}
	results = append(results, checkGoVersion())
	results = append(results, checkTinyGo())
	results = append(results, checkTailwind())
	results = append(results, checkProjectLayout()...)
	results = append(results, checkConfig())
	results = append(results, checkGitRepo())

	var ok, warn, fail int
	for _, r := range results {
		printResult(r)
		switch r.status {
		case statusOK:
			ok++
		case statusWarn:
			warn++
		case statusFail:
			fail++
		}
	}

	fmt.Println()
	fmt.Printf("  %s%d passed%s  ", colGreen, ok, colReset)
	if warn > 0 {
		fmt.Printf("%s%d warning%s  ", colYellow, warn, colReset)
	}
	if fail > 0 {
		fmt.Printf("%s%d failed%s", colRed, fail, colReset)
	}
	fmt.Println()
	fmt.Println()

	if fail > 0 {
		os.Exit(1)
	}
}

func printResult(r doctorResult) {
	var marker, label string
	switch r.status {
	case statusOK:
		marker = colGreen + "✓" + colReset
		label = r.label
	case statusWarn:
		marker = colYellow + "⚠" + colReset
		label = r.label
	case statusFail:
		marker = colRed + "✕" + colReset
		label = r.label
	default:
		marker = colDim + "●" + colReset
		label = r.label
	}
	fmt.Printf("  %s  %-28s  %s%s%s\n", marker, label, colDim, r.detail, colReset)
	if r.hint != "" {
		fmt.Printf("       %s↳ %s%s\n", colDim, r.hint, colReset)
	}
}

func checkGoVersion() doctorResult {
	r := doctorResult{label: "Go runtime"}
	r.detail = runtime.Version() + " on " + runtime.GOOS + "/" + runtime.GOARCH
	min := "go1.25"
	if !strings.HasPrefix(runtime.Version(), "go1.") {
		r.status = statusWarn
		r.hint = "unrecognised Go version string"
		return r
	}
	if compareGo(runtime.Version(), min) < 0 {
		r.status = statusFail
		r.hint = "Nguyen.go requires " + min + " or newer"
		return r
	}
	r.status = statusOK
	return r
}

func compareGo(have, min string) int {
	have = strings.TrimPrefix(have, "go")
	min = strings.TrimPrefix(min, "go")
	hp := strings.Split(have, ".")
	mp := strings.Split(min, ".")
	for i := 0; i < len(mp); i++ {
		var hv, mv int
		if i < len(hp) {
			fmt.Sscanf(hp[i], "%d", &hv)
		}
		fmt.Sscanf(mp[i], "%d", &mv)
		if hv != mv {
			return hv - mv
		}
	}
	return 0
}

func checkTinyGo() doctorResult {
	r := doctorResult{label: "TinyGo"}
	if path, err := exec.LookPath("tinygo"); err == nil {
		out, _ := exec.Command(path, "version").CombinedOutput()
		r.detail = strings.TrimSpace(string(out))
		if r.detail == "" {
			r.detail = path
		}
		r.status = statusOK
		return r
	}
	r.detail = "not found in PATH"
	r.status = statusWarn
	r.hint = "WASM compilation needs TinyGo. Install from https://tinygo.org/getting-started/install/ — SSR works without it."
	return r
}

func checkTailwind() doctorResult {
	r := doctorResult{label: "Tailwind binary"}
	candidates := []string{"tailwindcss", "tailwindcss.exe"}
	dirs := []string{".", "bin", ".nguyen"}
	cwd, _ := os.Getwd()
	for _, d := range dirs {
		for _, n := range candidates {
			p := filepath.Join(cwd, d, n)
			if _, err := os.Stat(p); err == nil {
				r.detail = p
				r.status = statusOK
				return r
			}
		}
	}
	if path, err := exec.LookPath("tailwindcss"); err == nil {
		r.detail = path
		r.status = statusOK
		return r
	}
	r.detail = "not present (optional)"
	r.status = statusInfo
	r.hint = "Run 'nguyen create my-app --tailwind' to scaffold a Tailwind-enabled project"
	return r
}

func checkProjectLayout() []doctorResult {
	results := []doctorResult{}
	cwd, _ := os.Getwd()
	check := func(name, dir string) doctorResult {
		r := doctorResult{label: name}
		full := filepath.Join(cwd, dir)
		if info, err := os.Stat(full); err == nil && info.IsDir() {
			count, _ := countFiles(full)
			r.detail = fmt.Sprintf("%s (%d file(s))", dir, count)
			r.status = statusOK
			return r
		}
		r.detail = "missing"
		r.status = statusWarn
		r.hint = fmt.Sprintf("create %s/ — see docs/getting-started.md", dir)
		return r
	}
	results = append(results,
		check("pages/ directory", "pages"),
		check("public/ directory", "public"),
		check("styles/ directory", "styles"),
	)
	return results
}

func countFiles(root string) (int, error) {
	var n int
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			n++
		}
		return nil
	})
	return n, err
}

func checkConfig() doctorResult {
	r := doctorResult{label: "config/nguyen.config.yml"}
	cwd, _ := os.Getwd()
	configs := []string{
		filepath.Join(cwd, "config", "nguyen.config.yml"),
		filepath.Join(cwd, "nguyen.config.yml"),
	}
	for _, p := range configs {
		if _, err := os.Stat(p); err == nil {
			r.detail = strings.TrimPrefix(p, cwd+string(os.PathSeparator))
			r.status = statusOK
			return r
		}
	}
	r.detail = "not found (defaults will be used)"
	r.status = statusInfo
	r.hint = "create config/nguyen.config.yml to customise server, render, GEO, PWA settings"
	return r
}

func checkGitRepo() doctorResult {
	r := doctorResult{label: "Git repository"}
	cwd, _ := os.Getwd()
	gitDir := filepath.Join(cwd, ".git")
	if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
		r.detail = "initialised"
		r.status = statusOK
		return r
	}
	r.detail = "not a git repo"
	r.status = statusInfo
	r.hint = "run 'git init' to enable reproducible builds + version stamping"
	return r
}
