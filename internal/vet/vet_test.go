package vet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeGox(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestVetEmptyDir(t *testing.T) {
	dir := t.TempDir()
	issues, err := Vet(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(issues))
	}
}

func TestVetContextFirstParam(t *testing.T) {
	dir := t.TempDir()
	writeGox(t, dir, "page.gox", `---
import "context"

func Load(name string) string { return name }
func Action(ctx context.Context, form FormData) error { return nil }
---
<h1>hi</h1>
`)
	issues, _ := Vet(dir, []Rule{ruleContextFirstParam()})
	if len(issues) != 1 {
		t.Fatalf("want 1 issue got %d: %#v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Message, "Load") {
		t.Fatalf("unexpected message: %s", issues[0].Message)
	}
}

func TestVetTimeSleep(t *testing.T) {
	dir := t.TempDir()
	writeGox(t, dir, "page.gox", `---
import "time"

func Load() {
	time.Sleep(time.Second)
}
---
<h1>hi</h1>
`)
	issues, _ := Vet(dir, []Rule{ruleNoTimeSleepInHandlers()})
	if len(issues) != 1 {
		t.Fatalf("want 1 issue got %d", len(issues))
	}
}

func TestVetGoroutineCancel(t *testing.T) {
	dir := t.TempDir()
	writeGox(t, dir, "page.gox", `---
func bad() {
	go func() {
		work()
	}()
}

func good(ctx Context) {
	go func() {
		select { case <-ctx.Done(): }
	}()
}
---
`)
	issues, _ := Vet(dir, []Rule{ruleGoroutineCancellation()})
	if len(issues) != 1 {
		t.Fatalf("want 1 issue got %d", len(issues))
	}
}

func TestVetSecrets(t *testing.T) {
	dir := t.TempDir()
	writeGox(t, dir, "page.gox", `---
const apiKey = "ABCDEF1234567890XYZ"
const password = "supersecret123456"
---
`)
	issues, _ := Vet(dir, []Rule{ruleNoHardcodedSecrets()})
	if len(issues) < 1 {
		t.Fatalf("expected at least 1 issue, got %d", len(issues))
	}
}

func TestVetFmtPrint(t *testing.T) {
	dir := t.TempDir()
	writeGox(t, dir, "page.gox", `---
import "fmt"

func Load() {
	fmt.Println("debug")
}
---
`)
	issues, _ := Vet(dir, []Rule{ruleNoFmtPrintInHandlers()})
	if len(issues) != 1 {
		t.Fatalf("want 1 issue got %d", len(issues))
	}
}

func TestVetEventHandlerExists(t *testing.T) {
	dir := t.TempDir()
	writeGox(t, dir, "page.gox", `---
func known() {}
---
<button @click="known()">ok</button>
<button @click="missing()">bad</button>
`)
	issues, _ := Vet(dir, []Rule{ruleEventHandlerExists()})
	if len(issues) != 1 {
		t.Fatalf("want 1 issue got %d: %v", len(issues), issues)
	}
	if !strings.Contains(issues[0].Message, "missing") {
		t.Fatalf("unexpected: %s", issues[0].Message)
	}
}

func TestSplitFrontmatter(t *testing.T) {
	in := "---\nfoo := 1\n---\n<h1>hi</h1>\n"
	out := splitFrontmatter(in)
	if !strings.Contains(out[0], "foo := 1") {
		t.Fatalf("frontmatter wrong: %q", out[0])
	}
	if !strings.Contains(out[1], "<h1>hi</h1>") {
		t.Fatalf("template wrong: %q", out[1])
	}
}

func TestSplitFrontmatterNone(t *testing.T) {
	in := "<h1>just a template</h1>"
	out := splitFrontmatter(in)
	if out[0] != "" || out[1] != "" {
		t.Fatalf("expected zero values: %q %q", out[0], out[1])
	}
}
