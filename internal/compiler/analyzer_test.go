package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeWASMRejectsInvalidFile(t *testing.T) {
	dir := t.TempDir()
	wasmPath := filepath.Join(dir, "invalid.wasm")
	if err := os.WriteFile(wasmPath, []byte{0x00, 0x61, 0x73}, 0644); err != nil {
		t.Fatalf("write invalid wasm: %v", err)
	}

	report, err := AnalyzeWASM(wasmPath)
	if err == nil {
		t.Fatalf("expected error for invalid wasm")
	}
	if report == nil {
		t.Fatalf("expected partial report even on error")
	}
}

func TestBuildReportHTMLIncludesTitleAndTotal(t *testing.T) {
	report := &SizeReport{
		Path:      "test.wasm",
		TotalSize: 2048,
		Sections: []SectionSize{
			{Name: "code", Size: 1024},
			{Name: "data", Size: 1024},
		},
		Functions: []FuncSize{
			{Name: "func_a", Size: 600, Offset: 10},
			{Name: "func_b", Size: 400, Offset: 20},
		},
	}

	html := BuildReportHTML(report)
	if !strings.Contains(html, "Nguyen.go Bundle Analyzer") {
		t.Fatalf("missing analyzer title")
	}
	if !strings.Contains(html, "Total") {
		t.Fatalf("missing total section")
	}
	if !strings.Contains(html, "Top Functions") {
		t.Fatalf("missing functions section")
	}
}
