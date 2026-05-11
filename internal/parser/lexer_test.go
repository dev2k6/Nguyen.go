package parser

import (
	"strings"
	"testing"
)

func TestParse_NoFrontmatter(t *testing.T) {
	content := "<html><body>Hello World</body></html>"
	result, err := parseString(content, "test.nguyen")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.GoCode != "" {
		t.Errorf("GoCode should be empty, got: %s", result.GoCode)
	}
	if result.HTMLTemplate != content {
		t.Errorf("HTMLTemplate should be entire content, got: %s", result.HTMLTemplate)
	}
}

func TestParse_WithFrontmatter(t *testing.T) {
	content := `---
count := 0
func increment() { count++ }
---
<html>
<body>Hello</body>
</html>`
	result, err := parseString(content, "test.nguyen")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !strings.Contains(result.GoCode, "count := 0") {
		t.Errorf("GoCode should contain 'count := 0', got: %s", result.GoCode)
	}
	if !strings.Contains(result.HTMLTemplate, "<html>") {
		t.Errorf("HTMLTemplate should contain '<html>', got: %s", result.HTMLTemplate)
	}
}

func TestParse_MissingClosingFrontmatter(t *testing.T) {
	content := `---
count := 0
<html>
<body>Hello</body>
</html>`
	_, err := parseString(content, "test.nguyen")
	if err == nil {
		t.Fatal("Expected error when missing closing --- for frontmatter")
	}
}

func TestParse_EmptyFrontmatter(t *testing.T) {
	content := `---
---
<html>
<body>Hello</body>
</html>`
	result, err := parseString(content, "test.nguyen")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.GoCode != "" {
		t.Errorf("GoCode should be empty with empty frontmatter, got: %s", result.GoCode)
	}
}

func TestName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"pages/index.nguyen", "index"},
		{"pages/product.nguyen", "product"},
		{"pages\\about.nguyen", "about"},
		{"/project/pages/home.nguyen", "home"},
	}

	for _, tc := range tests {
		f := &File{Path: tc.path}
		result := f.Name()
		if result != tc.expected {
			t.Errorf("Name(%s) = %s, expected %s", tc.path, result, tc.expected)
		}
	}
}

func TestPackageName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"pages/index.nguyen", "page_index"},
		{"pages/my-product.nguyen", "page_my_product"},
		{"pages/[id].nguyen", "page_id"},
	}

	for _, tc := range tests {
		f := &File{Path: tc.path}
		result := f.PackageName()
		if result != tc.expected {
			t.Errorf("PackageName(%s) = %s, expected %s", tc.path, result, tc.expected)
		}
	}
}
