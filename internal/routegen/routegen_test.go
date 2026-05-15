package routegen

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePage(t *testing.T, dir, name string) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("---\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateBasic(t *testing.T) {
	dir := t.TempDir()
	writePage(t, dir, "index.gox")
	writePage(t, dir, "about.gox")
	writePage(t, dir, "blog/index.gox")
	writePage(t, dir, "blog/[slug].gox")
	writePage(t, dir, "shop/[...path].gox")

	out := filepath.Join(t.TempDir(), "routes.gen.go")
	n, err := Generate(Options{
		PagesDir:   dir,
		OutputPath: out,
		Package:    "routes",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if n < 5 {
		t.Fatalf("expected at least 5 routes, got %d", n)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)

	for _, want := range []string{
		"package routes",
		"DO NOT EDIT",
		"var All = []string{",
		"func Index() string { return \"/\" }",
		"func About() string",
		"func BlogBySlug(slug string) string",
		"func ShopWildcard",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("missing %q in:\n%s", want, src)
		}
	}

	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "routes.gen.go", src, 0); err != nil {
		t.Fatalf("generated file does not parse: %v\n%s", err, src)
	}
}

func TestGenerateDeterministic(t *testing.T) {
	dir := t.TempDir()
	writePage(t, dir, "index.gox")
	writePage(t, dir, "blog/[slug].gox")
	writePage(t, dir, "user/[id].gox")

	out1 := filepath.Join(t.TempDir(), "a.go")
	out2 := filepath.Join(t.TempDir(), "b.go")
	if _, err := Generate(Options{PagesDir: dir, OutputPath: out1}); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(Options{PagesDir: dir, OutputPath: out2}); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(out1)
	b, _ := os.ReadFile(out2)
	if string(a) != string(b) {
		t.Fatalf("two generations differ:\n%s\n---\n%s", a, b)
	}
}
