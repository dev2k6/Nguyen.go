package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Name == "" {
		t.Error("Name should not be empty")
	}
	if cfg.Server.Port != 3000 {
		t.Errorf("default port should be 3000, got %d", cfg.Server.Port)
	}
	if cfg.Render.Mode != "csr" {
		t.Errorf("default render mode should be csr, got %s", cfg.Render.Mode)
	}
}

func TestServerAddress(t *testing.T) {
	cfg := ServerConfig{Host: "localhost", Port: 3000}
	if cfg.Address() != "localhost:3000" {
		t.Errorf("expected localhost:3000, got %s", cfg.Address())
	}
}

func TestServerAddress_Defaults(t *testing.T) {
	cfg := ServerConfig{}
	if cfg.Address() != "localhost:3000" {
		t.Errorf("expected localhost:3000, got %s", cfg.Address())
	}
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load("nonexistent_file.yml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "nguyen-app" {
		t.Errorf("expected default name, got %s", cfg.Name)
	}
}

func TestLoad_Valid(t *testing.T) {
	// Write a minimal config file
	yaml := `
name: test-app
version: 1.0.0
server:
  port: 4000
render:
  mode: ssr
`
	f, err := os.CreateTemp("", "nguyen-config-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(yaml)
	f.Close()

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "test-app" {
		t.Errorf("expected test-app, got %s", cfg.Name)
	}
	if cfg.Server.Port != 4000 {
		t.Errorf("expected port 4000, got %d", cfg.Server.Port)
	}
	if cfg.Render.Mode != "ssr" {
		t.Errorf("expected ssr mode, got %s", cfg.Render.Mode)
	}
}

func TestLoad_InvalidRenderMode(t *testing.T) {
	yaml := `
render:
  mode: invalid
`
	f, err := os.CreateTemp("", "nguyen-config-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(yaml)
	f.Close()

	_, err = Load(f.Name())
	if err == nil {
		t.Error("expected error for invalid render mode")
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	yaml := `
server:
  port: 99999
`
	f, err := os.CreateTemp("", "nguyen-config-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(yaml)
	f.Close()

	_, err = Load(f.Name())
	if err == nil {
		t.Error("expected error for invalid port")
	}
}
