package pwa

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dev2k6/Nguyen.go/internal/config"
)

func TestGenerateManifest(t *testing.T) {
	cfg := config.DefaultConfig()
	m, err := GenerateManifest(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name != "nguyen-app" {
		t.Errorf("expected name nguyen-app, got %s", m.Name)
	}
	if m.ShortName != "Nguyen" {
		t.Errorf("expected short_name Nguyen, got %s", m.ShortName)
	}
	if m.StartURL != "/" {
		t.Errorf("expected start_url /, got %s", m.StartURL)
	}
	if len(m.Icons) != 2 {
		t.Errorf("expected 2 icons, got %d", len(m.Icons))
	}
}

func TestGenerateManifest_Disabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.PWA.Enabled = false
	_, err := GenerateManifest(cfg)
	if err == nil {
		t.Error("expected error when PWA disabled")
	}
}

func TestWriteManifest(t *testing.T) {
	cfg := config.DefaultConfig()
	dir := t.TempDir()
	if err := WriteManifest(cfg, dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	path := filepath.Join(dir, "manifest.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("manifest.json not created")
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "nguyen-app") {
		t.Error("manifest missing app name")
	}
}

func TestGenerateServiceWorker(t *testing.T) {
	cfg := config.DefaultConfig()
	assets := []string{"/chunks/index.wasm", "/chunks/about.wasm"}
	sw := GenerateServiceWorker(cfg, assets)
	if !strings.Contains(sw, "nguyen-cache") {
		t.Error("SW missing cache name")
	}
	if !strings.Contains(sw, "/chunks/index.wasm") {
		t.Error("SW missing asset")
	}
	if !strings.Contains(sw, "install") {
		t.Error("SW missing install event")
	}
	if !strings.Contains(sw, "fetch") {
		t.Error("SW missing fetch event")
	}
}

func TestGenerateSWRegister(t *testing.T) {
	reg := GenerateSWRegister()
	if !strings.Contains(reg, "serviceWorker") {
		t.Error("registration script missing serviceWorker reference")
	}
	if !strings.Contains(reg, "/sw.js") {
		t.Error("registration script missing sw.js path")
	}
}

func TestWriteAll(t *testing.T) {
	cfg := config.DefaultConfig()
	dir := t.TempDir()
	if err := WriteAll(cfg, dir, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		t.Error("manifest.json not written")
	}
	if _, err := os.Stat(filepath.Join(dir, "sw.js")); err != nil {
		t.Error("sw.js not written")
	}
}

func TestWriteAll_Disabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.PWA.Enabled = false
	dir := t.TempDir()
	if err := WriteAll(cfg, dir, nil); err != nil {
		t.Fatalf("expected no-op when disabled, got: %v", err)
	}
}
