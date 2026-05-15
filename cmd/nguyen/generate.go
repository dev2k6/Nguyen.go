package main

import (
	"fmt"
	"os"

	"github.com/dev2k6/Nguyen.go/internal/routegen"
	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Code generation utilities (routes, content, rpc)",
	Long: `Generate emits Go source files derived from project state. Sub-commands:

  nguyen generate routes   typed accessors for every .gox route

Generated files live next to your code with the .gen.go suffix and
are deterministic — running generate twice produces byte-identical
output, so they diff cleanly in code review.`,
}

var generateRoutesCmd = &cobra.Command{
	Use:   "routes",
	Short: "Generate typed route accessors from .gox files",
	Run: func(cmd *cobra.Command, args []string) {
		runGenerateRoutes()
	},
}

var (
	flagGenRoutesPages   string
	flagGenRoutesOutput  string
	flagGenRoutesPackage string
)

func init() {
	generateRoutesCmd.Flags().StringVar(&flagGenRoutesPages, "pages", "pages", "Pages directory")
	generateRoutesCmd.Flags().StringVar(&flagGenRoutesOutput, "out", "routes/routes.gen.go", "Output path for generated file")
	generateRoutesCmd.Flags().StringVar(&flagGenRoutesPackage, "package", "routes", "Package name")
	generateCmd.AddCommand(generateRoutesCmd)
	RootCmd.AddCommand(generateCmd)
}

func runGenerateRoutes() {
	fmt.Println()
	fmt.Printf("%s  ⬡ Nguyen.go — Generate Routes%s\n", colBold, colReset)
	fmt.Println()

	if _, err := os.Stat(flagGenRoutesPages); os.IsNotExist(err) {
		fmt.Printf("  %s✕%s Pages directory not found: %s\n\n", colRed, colReset, flagGenRoutesPages)
		os.Exit(1)
	}

	n, err := routegen.Generate(routegen.Options{
		PagesDir:   flagGenRoutesPages,
		OutputPath: flagGenRoutesOutput,
		Package:    flagGenRoutesPackage,
	})
	if err != nil {
		fmt.Printf("  %s✕%s %v\n\n", colRed, colReset, err)
		os.Exit(1)
	}
	fmt.Printf("  %s✓%s wrote %d route accessor(s) → %s%s%s\n",
		colGreen, colReset, n, colCyan, flagGenRoutesOutput, colReset)
	fmt.Println()
}
