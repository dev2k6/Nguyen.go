package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/dev2k6/Nguyen.go/internal/vet"
	"github.com/spf13/cobra"
)

var vetCmd = &cobra.Command{
	Use:   "vet",
	Short: "Run static analysis on .gox files",
	Long: `Vet walks every .gox file under --pages and applies a set of
rules: context.Context propagation, blocking time.Sleep,
goroutines that ignore cancellation, hardcoded secrets,
unstructured fmt.Println, and missing event handler bindings.

Errors fail vet (exit code 1). Warnings do not. Use --strict to
treat warnings as errors for CI gating.`,
	Run: func(cmd *cobra.Command, args []string) {
		runVet()
	},
}

var (
	flagVetPages  string
	flagVetStrict bool
)

func init() {
	vetCmd.Flags().StringVar(&flagVetPages, "pages", "pages", "Pages directory")
	vetCmd.Flags().BoolVar(&flagVetStrict, "strict", false, "Treat warnings as errors")
	RootCmd.AddCommand(vetCmd)
}

func runVet() {
	fmt.Println()
	fmt.Printf("%s  ⬡ Nguyen.go — Vet%s\n", colBold, colReset)
	fmt.Println()

	if _, err := os.Stat(flagVetPages); os.IsNotExist(err) {
		fmt.Printf("  %s✕%s Pages directory not found: %s\n\n", colRed, colReset, flagVetPages)
		os.Exit(1)
	}

	issues, err := vet.Vet(flagVetPages, nil)
	if err != nil {
		fmt.Printf("  %s✕%s vet failed: %v\n", colRed, colReset, err)
		os.Exit(1)
	}

	sort.Slice(issues, func(i, j int) bool {
		if issues[i].File != issues[j].File {
			return issues[i].File < issues[j].File
		}
		return issues[i].Line < issues[j].Line
	})

	var warnings, errors int
	for _, iss := range issues {
		marker := colYellow + "⚠" + colReset
		if iss.Severity == vet.SeverityError {
			marker = colRed + "✕" + colReset
			errors++
		} else {
			warnings++
		}
		fmt.Printf("  %s  %s:%d  %s[%s]%s %s\n",
			marker, iss.File, iss.Line, colDim, iss.Rule, colReset, iss.Message,
		)
	}

	fmt.Println()
	if errors > 0 || warnings > 0 {
		fmt.Printf("  %s%d error(s)%s  %s%d warning(s)%s\n",
			colRed, errors, colReset, colYellow, warnings, colReset)
	} else {
		fmt.Printf("  %s✓ No issues found%s\n", colGreen, colReset)
	}
	fmt.Println()

	exit := 0
	if errors > 0 {
		exit = 1
	}
	if flagVetStrict && warnings > 0 {
		exit = 1
	}
	os.Exit(exit)
}
