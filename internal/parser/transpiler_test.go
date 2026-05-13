package parser

import (
	"strings"
	"testing"
)

func TestExtractStateVars(t *testing.T) {
	template := `<div>Hello {name}</div><span>Count: {count}</span>`
	vars := extractStateVars(template)
	if len(vars) != 2 {
		t.Fatalf("Expected 2 variables, got %d: %v", len(vars), vars)
	}
	if vars[0] != "name" || vars[1] != "count" {
		t.Errorf("Variable list incorrect: %v", vars)
	}
}

func TestExtractStateVars_NoVariables(t *testing.T) {
	template := `<div>Hello World</div>`
	vars := extractStateVars(template)
	if len(vars) != 0 {
		t.Errorf("Expected 0 variables, got %d: %v", len(vars), vars)
	}
}

func TestExtractStateVars_Duplicates(t *testing.T) {
	template := `<div>{count}{count}</div><span>{count}</span>`
	vars := extractStateVars(template)
	if len(vars) != 1 {
		t.Errorf("Expected 1 variable (deduped), got %d: %v", len(vars), vars)
	}
}

func TestExtractEventHandlers(t *testing.T) {
	template := `<button @click="increment()">+</button><button @click="decrement()">-</button>`
	handlers := extractEventHandlers(template)
	if len(handlers) != 2 {
		t.Fatalf("Expected 2 handlers, got %d: %v", len(handlers), handlers)
	}
	if handlers[0] != "increment" || handlers[1] != "decrement" {
		t.Errorf("Handler list incorrect: %v", handlers)
	}
}

func TestExtractEventHandlers_NoEvents(t *testing.T) {
	template := `<button>Click me</button>`
	handlers := extractEventHandlers(template)
	if len(handlers) != 0 {
		t.Errorf("Expected 0 handlers, got %d: %v", len(handlers), handlers)
	}
}

func TestTranspile_CreatesPackage(t *testing.T) {
	f := &File{
		Path:         "pages/index.gox",
		GoCode:       "count := 0",
		HTMLTemplate: "<div>Hello</div>",
	}
	info := Transpile(f)
	if info.PackageName != "page_index" {
		t.Errorf("PackageName incorrect: %s", info.PackageName)
	}
	// Generated WASM source is always package main (required by TinyGo wasm target)
	if !strings.Contains(info.SourceCode, "package main") {
		t.Errorf("SourceCode missing package main declaration")
	}
	if !strings.Contains(info.SourceCode, "github.com/dev2k6/Nguyen.go/pkg/core") {
		t.Errorf("SourceCode missing core import")
	}
}

func TestTranspile_WithGoCode(t *testing.T) {
	f := &File{
		Path:         "pages/counter.gox",
		GoCode:       "count := 0\nfunc increment() {}",
		HTMLTemplate: `<button @click="increment()">+</button>`,
	}
	info := Transpile(f)
	// At package level, := is converted to var
	if !strings.Contains(info.SourceCode, "var count = 0") {
		t.Errorf("SourceCode missing user GoCode (expected var count = 0)")
	}
	if !strings.Contains(info.SourceCode, "func increment()") {
		t.Errorf("SourceCode missing increment function")
	}
}

func TestTranspile_DetectsStateVars(t *testing.T) {
	f := &File{
		Path:         "pages/counter.gox",
		GoCode:       "count := 0",
		HTMLTemplate: "<div>Count: {count}</div>",
	}
	info := Transpile(f)
	if len(info.StateVars) != 1 || info.StateVars[0] != "count" {
		t.Errorf("StateVars incorrect: %v", info.StateVars)
	}
}

func TestTranspile_DetectsEventHandlers(t *testing.T) {
	f := &File{
		Path:         "pages/button.gox",
		GoCode:       "func increment() {}",
		HTMLTemplate: `<button @click="increment()">+</button>`,
	}
	info := Transpile(f)
	if len(info.EventHandlers) != 1 || info.EventHandlers[0] != "increment" {
		t.Errorf("EventHandlers incorrect: %v", info.EventHandlers)
	}
}

func TestTranspile_GeneratesRenderFunction(t *testing.T) {
	f := &File{
		Path:         "pages/index.gox",
		GoCode:       "",
		HTMLTemplate: "<div>Hello</div>",
	}
	info := Transpile(f)
	if !strings.Contains(info.SourceCode, "func Render") {
		t.Errorf("SourceCode missing Render function")
	}
	if !strings.Contains(info.SourceCode, "core.H(") {
		t.Errorf("SourceCode missing core.H call (VNode-based)")
	}
}
