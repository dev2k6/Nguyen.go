package render

import (
	"strings"
	"testing"
)

func TestBuildStreamingShell(t *testing.T) {
	result := &Result{
		Meta:    map[string]string{"title": "Test"},
		Version: "1.0",
	}
	shell := buildStreamingShell(result, "1.0")
	if !strings.Contains(shell, "<!DOCTYPE html>") {
		t.Error("shell missing DOCTYPE")
	}
	if !strings.Contains(shell, `<title>Test</title>`) {
		t.Error("shell missing title")
	}
	if !strings.Contains(shell, `id="app"`) {
		t.Error("shell missing app container")
	}
	if !strings.Contains(shell, `manifest.json`) {
		t.Error("shell missing manifest link")
	}
}

func TestBuildDataInjectScript(t *testing.T) {
	data := map[string]interface{}{
		"title": "Hello",
		"count": 42,
		"flag":  true,
	}
	script := buildDataInjectScript(data)
	if !strings.Contains(script, "window.__nguyenPageData") {
		t.Error("script missing __nguyenPageData")
	}
	if !strings.Contains(script, `"title"`) {
		t.Error("script missing title key")
	}
	if !strings.Contains(script, "42") {
		t.Error("script missing count value")
	}
}

func TestStripDocShell(t *testing.T) {
	html := `<!DOCTYPE html>
<html lang="en">
<head><title>Test</title></head>
<body><div>Content</div></body>
</html>`
	stripped := stripDocShell(html)
	if strings.Contains(stripped, "<!DOCTYPE") {
		t.Error("stripDocShell left DOCTYPE")
	}
	if strings.Contains(stripped, "<html") {
		t.Error("stripDocShell left <html>")
	}
	if strings.Contains(stripped, "<head>") {
		t.Error("stripDocShell left <head>")
	}
	if strings.Contains(stripped, "<body>") {
		t.Error("stripDocShell left <body>")
	}
	if !strings.Contains(stripped, "<div>Content</div>") {
		t.Error("stripDocShell removed actual content")
	}
}

func TestStripTag(t *testing.T) {
	html := `<div class="x">content</div>`
	stripped := stripTag(html, "div")
	if strings.Contains(stripped, "<div") {
		t.Error("stripTag left opening tag")
	}
	if strings.Contains(stripped, "</div>") {
		t.Error("stripTag left closing tag")
	}
	if !strings.Contains(stripped, "content") {
		t.Error("stripTag removed content")
	}
}
