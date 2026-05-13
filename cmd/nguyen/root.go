package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dev2k6/Nguyen.go/internal/version"
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "nguyen",
	Short: "Nguyen.go - Full-stack Web Framework",
	Long: fmt.Sprintf(`%s
Nguyen.go is a Full-stack Web Framework combining Go backend with
WebAssembly (TinyGo) for client-side rendering. Supports .gox syntax
with advanced rendering features (SSR, ISR, CSR).

Author: Thái Nguyên <thainguyen.junior@gmail.com>`, asciiArt()),
	Version: version.Version,
}

var flagRoot string

func init() {
	RootCmd.PersistentFlags().StringVar(&flagRoot, "root", "", "Project root directory (chdir before running)")
	cobra.OnInitialize(applyRoot)

	RootCmd.AddCommand(devCmd)
	RootCmd.AddCommand(buildCmd)
	RootCmd.AddCommand(checkCmd)
	RootCmd.AddCommand(createCmd)
	RootCmd.AddCommand(startCmd)
	RootCmd.AddCommand(exportCmd)
}

// applyRoot changes CWD to the resolved --root before a subcommand executes.
// All downstream paths ("pages", "config/nguyen.config.yml", "public", etc.)
// are relative to CWD, so this makes CLI invocations reproducible from any shell.
func applyRoot() {
	if flagRoot == "" {
		return
	}
	abs, err := filepath.Abs(flagRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ✕ Invalid --root: %v\n", err)
		os.Exit(1)
	}
	if err := os.Chdir(abs); err != nil {
		fmt.Fprintf(os.Stderr, "  ✕ Cannot chdir to --root %s: %v\n", abs, err)
		os.Exit(1)
	}
}

// asciiArt returns the Nguyen.go ASCII art banner
func asciiArt() string {
	return `
   ███╗   ██╗ ██████╗ ██╗   ██╗██╗   ██╗███████╗███╗   ██╗
   ████╗  ██║██╔════╝ ██║   ██║╚██╗ ██╔╝██╔════╝████╗  ██║
   ██╔██╗ ██║██║  ███╗██║   ██║ ╚████╔╝ █████╗  ██╔██╗ ██║
   ██║╚██╗██║██║   ██║██║   ██║  ╚██╔╝  ██╔══╝  ██║╚██╗██║
   ██║ ╚████║╚██████╔╝╚██████╔╝   ██║   ███████╗██║ ╚████║
   ╚═╝  ╚═══╝ ╚═════╝  ╚═════╝    ╚═╝   ╚══════╝╚═╝  ╚═══╝
`
}
