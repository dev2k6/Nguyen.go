package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var distCmd = &cobra.Command{
	Use:   "dist",
	Short: "Cross-compile reproducible single-binary releases",
	Long: `Dist produces production binaries for one or more target
platforms using -trimpath and stripped buildid flags so two
builds from the same source tree yield byte-identical output.
Each target's binary is written to dist/<os>-<arch>/<name>.

Example:
  nguyen dist --targets linux/amd64,linux/arm64,darwin/arm64
  nguyen dist --reproducible --targets linux/amd64`,
	Run: func(cmd *cobra.Command, args []string) {
		runDist()
	},
}

var (
	flagDistTargets      string
	flagDistOutput       string
	flagDistName         string
	flagDistMain         string
	flagDistReproducible bool
	flagDistLDFlags      string
)

func init() {
	distCmd.Flags().StringVar(&flagDistTargets, "targets", runtime.GOOS+"/"+runtime.GOARCH,
		"Comma-separated list of GOOS/GOARCH pairs (e.g. linux/amd64,linux/arm64,darwin/arm64)")
	distCmd.Flags().StringVar(&flagDistOutput, "output", "dist", "Output directory")
	distCmd.Flags().StringVar(&flagDistName, "name", "app", "Binary name (without .exe)")
	distCmd.Flags().StringVar(&flagDistMain, "main", "./cmd/server", "Path to main package")
	distCmd.Flags().BoolVar(&flagDistReproducible, "reproducible", true,
		"Strip path/buildid/timestamps so the same input produces byte-identical output")
	distCmd.Flags().StringVar(&flagDistLDFlags, "ldflags", "", "Extra ldflags appended after reproducible flags")
	RootCmd.AddCommand(distCmd)
}

type distTarget struct {
	OS   string
	Arch string
}

func parseTargets(spec string) ([]distTarget, error) {
	if spec == "" {
		return nil, fmt.Errorf("empty targets list")
	}
	var out []distTarget
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		bits := strings.Split(part, "/")
		if len(bits) != 2 || bits[0] == "" || bits[1] == "" {
			return nil, fmt.Errorf("invalid target %q (want goos/goarch)", part)
		}
		out = append(out, distTarget{OS: bits[0], Arch: bits[1]})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no targets parsed")
	}
	return out, nil
}

func runDist() {
	fmt.Println()
	fmt.Printf("%s  ⬡ Nguyen.go — Distribute%s\n", colBold, colReset)
	fmt.Println()

	targets, err := parseTargets(flagDistTargets)
	if err != nil {
		fmt.Printf("  %s✕%s %v\n\n", colRed, colReset, err)
		os.Exit(1)
	}

	if _, err := os.Stat(flagDistMain); os.IsNotExist(err) {
		fmt.Printf("  %s✕%s main package not found: %s\n", colRed, colReset, flagDistMain)
		fmt.Printf("  %s↳ pass --main to point at your cmd/<name>%s\n\n", colDim, colReset)
		os.Exit(1)
	}

	if err := os.MkdirAll(flagDistOutput, 0o755); err != nil {
		fmt.Printf("  %s✕%s mkdir: %v\n\n", colRed, colReset, err)
		os.Exit(1)
	}

	failed := 0
	for _, t := range targets {
		start := time.Now()
		out, err := buildOne(t)
		dur := time.Since(start)
		if err != nil {
			failed++
			fmt.Printf("  %s✕%s %s/%s  %s%v%s\n", colRed, colReset, t.OS, t.Arch, colDim, err, colReset)
			continue
		}
		size := fileSize(out)
		fmt.Printf("  %s✓%s %s/%s  %s%s%s  %s(%s, %.2fs)%s\n",
			colGreen, colReset, t.OS, t.Arch,
			colCyan, out, colReset,
			colDim, size, dur.Seconds(), colReset,
		)
	}

	fmt.Println()
	if failed > 0 {
		fmt.Printf("  %s%d target(s) failed%s\n\n", colRed, failed, colReset)
		os.Exit(1)
	}
	if flagDistReproducible {
		fmt.Printf("  %s✓ Reproducible flags applied%s\n", colGreen, colReset)
		fmt.Printf("  %s↳ verify with: sha256sum %s/*/{%s,%s.exe}%s\n", colDim, flagDistOutput, flagDistName, flagDistName, colReset)
		fmt.Println()
	}
}

func buildOne(t distTarget) (string, error) {
	binName := flagDistName
	if t.OS == "windows" {
		binName += ".exe"
	}
	outDir := filepath.Join(flagDistOutput, t.OS+"-"+t.Arch)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	outPath := filepath.Join(outDir, binName)

	args := []string{"build"}
	if flagDistReproducible {
		args = append(args, "-trimpath", "-buildvcs=false")
	}
	ldflags := ""
	if flagDistReproducible {
		ldflags = "-s -w -buildid="
	}
	if flagDistLDFlags != "" {
		if ldflags != "" {
			ldflags += " "
		}
		ldflags += flagDistLDFlags
	}
	if ldflags != "" {
		args = append(args, "-ldflags", ldflags)
	}
	args = append(args, "-o", outPath, flagDistMain)

	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(),
		"GOOS="+t.OS,
		"GOARCH="+t.Arch,
		"CGO_ENABLED=0",
	)
	if flagDistReproducible {
		// SOURCE_DATE_EPOCH is honoured by some link-time tools; clearing
		// it keeps cross-tool consistency. -trimpath + -buildid="" already
		// remove most non-determinism in standard Go builds.
		cmd.Env = append(cmd.Env, "SOURCE_DATE_EPOCH=0")
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return outPath, nil
}

func fileSize(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "?"
	}
	n := info.Size()
	switch {
	case n >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	case n >= 1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
