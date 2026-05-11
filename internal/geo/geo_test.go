package geo

import (
	"strings"
	"testing"

	"github.com/dev2k6/Nguyen.go/internal/config"
	"github.com/dev2k6/Nguyen.go/internal/router"
)

func TestArticleSchema(t *testing.T) {
	pg := &PageGEO{
		PageType:      "Article",
		Title:         "Test Article",
		Description:   "A test article description",
		DatePublished: "2025-01-15T10:00:00Z",
		DateModified:  "2025-05-08T14:00:00Z",
		Author:        "Test Author",
		Images:        []string{"/img/test.png"},
	}

	cfg := &config.GEOConfig{
		AuthorType:       "Person",
		OrganizationName: "Test Org",
		OrganizationURL:  "https://test.com",
	}

	schema := ArticleSchema(pg, cfg)

	if schema["@context"] != "https://schema.org" {
		t.Error("missing @context")
	}
	if schema["@type"] != "Article" {
		t.Errorf("expected type Article, got %v", schema["@type"])
	}
	if schema["headline"] != "Test Article" {
		t.Errorf("expected headline 'Test Article', got %v", schema["headline"])
	}

	author, ok := schema["author"].(map[string]interface{})
	if !ok {
		t.Error("author should be a map")
	} else if author["@type"] != "Person" {
		t.Errorf("author type should be Person, got %v", author["@type"])
	}

	publisher := schema["publisher"].(map[string]interface{})
	if publisher["name"] != "Test Org" {
		t.Errorf("publisher name incorrect: %v", publisher["name"])
	}
}

func TestFAQPageSchema(t *testing.T) {
	pg := &PageGEO{
		PageType: "FAQPage",
		Title:    "FAQ",
		Faqs: []FAQItem{
			{Question: "What is Go?", Answer: "Go is a programming language."},
			{Question: "What is WASM?", Answer: "WebAssembly."},
		},
	}

	cfg := config.DefaultConfig()
	schema := FAQPageSchema(pg, &cfg.GEO)

	entities, ok := schema["mainEntity"].([]map[string]interface{})
	if !ok {
		t.Fatal("mainEntity should be an array")
	}
	if len(entities) != 2 {
		t.Fatalf("expected 2 FAQs, got %d", len(entities))
	}
	if entities[0]["name"] != "What is Go?" {
		t.Errorf("first question wrong: %v", entities[0]["name"])
	}
}

func TestHowToSchema(t *testing.T) {
	pg := &PageGEO{
		PageType: "HowTo",
		Title:    "How to Build",
		HowToSteps: []HowToStep{
			{Name: "Install", Text: "Run setup command"},
			{Name: "Build", Text: "Run build command"},
		},
	}

	cfg := config.DefaultConfig()
	schema := HowToSchema(pg, &cfg.GEO)

	if schema["@type"] != "HowTo" {
		t.Errorf("expected HowTo type, got %v", schema["@type"])
	}

	steps, ok := schema["step"].([]map[string]interface{})
	if !ok {
		t.Fatal("step should be an array")
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
}

func TestBreadcrumbListSchema(t *testing.T) {
	pg := &PageGEO{
		Breadcrumbs: []BreadcrumbItem{
			{Name: "Home", URL: "/"},
			{Name: "Blog", URL: "/blog"},
		},
	}

	jsonld := BreadcrumbListSchema(pg)
	if !strings.Contains(jsonld, "BreadcrumbList") {
		t.Error("missing BreadcrumbList type")
	}
	if !strings.Contains(jsonld, "Home") {
		t.Error("missing Home breadcrumb")
	}
	if !strings.Contains(jsonld, "Blog") {
		t.Error("missing Blog breadcrumb")
	}
}

func TestOrganizationSchema(t *testing.T) {
	cfg := &config.GEOConfig{
		SiteName:         "Test Site",
		OrganizationName: "Test Org",
		OrganizationURL:  "https://test.com",
		SameAs:           []string{"https://twitter.com/test"},
	}

	jsonld := OrganizationSchema(cfg)
	if !strings.Contains(jsonld, "Organization") {
		t.Error("missing Organization type")
	}
	if !strings.Contains(jsonld, "Test Site") {
		t.Error("missing org name (SiteName takes priority over OrganizationName)")
	}
}

func TestWebSiteSchema(t *testing.T) {
	cfg := &config.GEOConfig{
		SiteName: "Test Site",
		SiteURL:  "https://test.com",
	}

	jsonld := WebSiteSchema(cfg)
	if !strings.Contains(jsonld, "WebSite") {
		t.Error("missing WebSite type")
	}
	if !strings.Contains(jsonld, "SearchAction") {
		t.Error("missing SearchAction")
	}
}

func TestGenerateSitemapXML(t *testing.T) {
	routes := []router.Route{
		{Pattern: "/", Priority: 0},
		{Pattern: "/about", Priority: 1},
		{Pattern: "/blog/:slug", Priority: 10},
	}

	cfg := &config.GEOConfig{
		SiteURL: "https://example.com",
		Sitemap: config.SitemapConfig{Enabled: true},
	}

	xml := GenerateSitemapXML(routes, cfg)
	if !strings.Contains(xml, "<?xml") {
		t.Error("missing XML declaration")
	}
	if !strings.Contains(xml, "https://example.com/") {
		t.Error("missing home URL")
	}
	if !strings.Contains(xml, "https://example.com/blog/:slug") {
		t.Error("missing blog route")
	}
}

func TestGenerateRobotsTxt(t *testing.T) {
	cfg := &config.GEOConfig{
		SiteName: "Test",
		SiteURL:  "https://test.com",
		Robots:   config.RobotsConfig{Enabled: true, AllowAIBots: false},
		Sitemap:  config.SitemapConfig{Enabled: true},
	}

	txt := GenerateRobotsTxt(cfg)
	if !strings.Contains(txt, "GPTBot") {
		t.Error("missing GPTBot directive")
	}
	if !strings.Contains(txt, "Disallow") {
		t.Error("AI bots should be disallowed by default")
	}
	if !strings.Contains(txt, "Sitemap") {
		t.Error("missing sitemap reference")
	}

	// Test with AI bots allowed
	cfg.Robots.AllowAIBots = true
	txt = GenerateRobotsTxt(cfg)
	if !strings.Contains(txt, "Allow: /") {
		t.Error("should have Allow when AI bots allowed")
	}
}

func TestComputePriority(t *testing.T) {
	tests := []struct {
		pattern string
		is404   bool
		want    float64
	}{
		{"/", false, 1.0},
		{"/about", false, 0.8},
		{"/blog/post", false, 0.6},
		{"/a/b/c/d", false, 0.4},
		{"/anything", true, 0.1},
	}

	for _, tc := range tests {
		got := ComputePriority(tc.pattern, tc.is404)
		if got != tc.want {
			t.Errorf("ComputePriority(%q, %v) = %v, want %v", tc.pattern, tc.is404, got, tc.want)
		}
	}
}

func TestBuildJSONLD(t *testing.T) {
	cfg := &config.GEOConfig{
		DefaultPageType:  "Article",
		AuthorType:       "Organization",
		OrganizationName: "Test Org",
		OrganizationURL:  "https://test.com",
	}

	pg := &PageGEO{
		PageType:      "Article",
		Title:         "Test",
		Description:   "A test",
		DatePublished: "2025-01-01T00:00:00Z",
		Author:        "Test Author",
	}

	jsonld, err := pg.BuildJSONLD(cfg)
	if err != nil {
		t.Fatalf("BuildJSONLD error: %v", err)
	}
	if !strings.Contains(jsonld, "https://schema.org") {
		t.Error("missing schema context")
	}
	if !strings.Contains(jsonld, "Article") {
		t.Error("missing Article type")
	}
}

func TestGenerateLLMtxt(t *testing.T) {
	routes := []router.Route{
		{Pattern: "/"},
		{Pattern: "/about"},
		{Pattern: "/.well-known/test", Is404: true},
	}

	cfg := &config.GEOConfig{
		SiteName: "Test Site",
		SiteURL:  "https://test.com",
		LLMTxt:   config.LLMTxtConfig{Enabled: true, MaxChars: 5000},
	}

	txt := GenerateLLMtxt(routes, nil, cfg)
	if !strings.Contains(txt, "Test Site") {
		t.Error("missing site name")
	}
	if !strings.Contains(txt, "https://test.com") {
		t.Error("missing site URL")
	}
	if !strings.Contains(txt, "## Pages") {
		t.Error("missing Pages section")
	}
	if strings.Contains(txt, ".well-known") {
		t.Error("should not include 404 routes")
	}
}

func TestGenerateAIBotTxt(t *testing.T) {
	cfg := &config.GEOConfig{
		SiteName: "Test",
		SiteURL:  "https://test.com",
	}

	txt := GenerateAIBotTxt(cfg)
	if !strings.Contains(txt, "AI Bot Access Policy") {
		t.Error("missing policy header")
	}
	if !strings.Contains(txt, "test.com") {
		t.Error("missing domain")
	}
}
