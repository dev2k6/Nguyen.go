package geo

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dev2k6/Nguyen.go/internal/config"
	"github.com/dev2k6/Nguyen.go/internal/router"
)

// PageGEO holds all GEO metadata for a single page
type PageGEO struct {
	PageType           string           `json:"pageType"` // "Article", "FAQPage", "HowTo", etc.
	Title              string           `json:"title"`
	Description        string           `json:"description"`
	DatePublished      string           `json:"datePublished"` // ISO 8601
	DateModified       string           `json:"dateModified"`  // ISO 8601
	Author             string           `json:"author"`
	SpeakableSelectors []string         `json:"speakable"` // CSS selectors
	Breadcrumbs        []BreadcrumbItem `json:"breadcrumbs"`
	SameAs             []string         `json:"sameAs"`
	Images             []string         `json:"images"`
	Tags               []string         `json:"tags"`
	Faqs               []FAQItem        `json:"faqs"`
	HowToSteps         []HowToStep      `json:"howToSteps"`
	Citations          []Citation       `json:"citations"`
}

// BreadcrumbItem represents a step in a breadcrumb trail
type BreadcrumbItem struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// FAQItem represents a question/answer pair
type FAQItem struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// HowToStep represents a step in a how-to guide
type HowToStep struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// Citation represents a source citation
type Citation struct {
	Claim string `json:"claim"`
	URL   string `json:"url"`
}

// SitemapEntry represents a single URL in the sitemap
type SitemapEntry struct {
	Loc        string  `xml:"loc"`
	LastMod    string  `xml:"lastmod,omitempty"`
	ChangeFreq string  `xml:"changefreq,omitempty"`
	Priority   float64 `xml:"priority,omitempty"`
}

// BuildJSONLD generates the JSON-LD string for a given PageGEO
func (p *PageGEO) BuildJSONLD(cfg *config.GEOConfig) (string, error) {
	if p == nil {
		return "", nil
	}
	if p.PageType == "" {
		p.PageType = cfg.DefaultPageType
	}
	if p.PageType == "" {
		p.PageType = "WebPage"
	}

	var ld map[string]interface{}

	switch p.PageType {
	case "Article", "BlogPosting", "TechArticle", "NewsArticle":
		ld = ArticleSchema(p, cfg)
	case "FAQPage":
		ld = FAQPageSchema(p, cfg)
	case "HowTo":
		ld = HowToSchema(p, cfg)
	default:
		ld = WebPageSchema(p, cfg)
	}

	// Add speakable if specified
	if len(p.SpeakableSelectors) > 0 {
		ld["speakable"] = map[string]interface{}{
			"@type":       "SpeakableSpecification",
			"cssSelector": p.SpeakableSelectors,
		}
	}

	// Add breadcrumb as separate entity if specified
	b, err := json.Marshal(ld)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// BuildBreadcrumbJSONLD generates breadcrumb JSON-LD separately
func (p *PageGEO) BuildBreadcrumbJSONLD() string {
	if p == nil || len(p.Breadcrumbs) == 0 {
		return ""
	}
	return BreadcrumbListSchema(p)
}

// BuildSiteJSONLD generates WebSite + Organization JSON-LD from config
func BuildSiteJSONLD(cfg *config.GEOConfig) string {
	org := OrganizationSchema(cfg)
	site := WebSiteSchema(cfg)
	// Merge into array
	return "[" + org + "," + site + "]"
}

// ComputePriority estimates priority based on route depth
func ComputePriority(pattern string, is404 bool) float64 {
	if is404 {
		return 0.1
	}
	// Count path segments (e.g. "/about"=1, "/blog/post"=2, "/"=0)
	parts := strings.Split(strings.Trim(pattern, "/"), "/")
	depth := len(parts)
	if pattern == "/" {
		depth = 0
	}
	if depth <= 0 {
		return 1.0
	}
	if depth <= 1 {
		return 0.8
	}
	if depth <= 2 {
		return 0.6
	}
	return 0.4
}

// ComputeChangeFreq estimates change frequency
func ComputeChangeFreq(pattern string) string {
	if strings.Contains(pattern, "blog") || strings.Contains(pattern, "post") {
		return "weekly"
	}
	if pattern == "/" {
		return "daily"
	}
	return "monthly"
}

// GenerateSitemapXML builds the full sitemap.xml string
func GenerateSitemapXML(routes []router.Route, cfg *config.GEOConfig) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")

	for _, r := range routes {
		if r.Is404 {
			continue
		}
		loc := cfg.SiteURL + r.Pattern
		priority := ComputePriority(r.Pattern, r.Is404)
		changefreq := ComputeChangeFreq(r.Pattern)

		sb.WriteString("  <url>\n")
		sb.WriteString(fmt.Sprintf("    <loc>%s</loc>\n", escapeXML(loc)))
		sb.WriteString(fmt.Sprintf("    <changefreq>%s</changefreq>\n", changefreq))
		sb.WriteString(fmt.Sprintf("    <priority>%.1f</priority>\n", priority))
		sb.WriteString("  </url>\n")
	}

	sb.WriteString("</urlset>\n")
	return sb.String()
}

// GenerateRobotsTxt builds robots.txt with AI crawler rules
func GenerateRobotsTxt(cfg *config.GEOConfig) string {
	var sb strings.Builder

	// Allow all for regular bots
	sb.WriteString("User-agent: *\n")
	sb.WriteString("Allow: /\n\n")

	// AI crawler directives
	aiBots := []string{
		"GPTBot", "ChatGPT-User", "Claude-Web", "PerplexityBot",
		"Bytespider", "Google-Extended", "Amazonbot", "cohere-ai",
	}

	for _, bot := range aiBots {
		sb.WriteString(fmt.Sprintf("User-agent: %s\n", bot))
		if cfg.Robots.AllowAIBots {
			sb.WriteString("Allow: /\n")
		} else {
			sb.WriteString("Disallow: /\n")
		}
		sb.WriteString("\n")
	}

	// Sitemap
	if cfg.Sitemap.Enabled {
		sb.WriteString(fmt.Sprintf("Sitemap: %s/sitemap.xml\n", cfg.SiteURL))
	}

	return sb.String()
}

// GenerateAIBotTxt builds /.well-known/ai-bot.txt
func GenerateAIBotTxt(cfg *config.GEOConfig) string {
	return fmt.Sprintf(`# AI Bot Access Policy for %s
# Generated by Nguyen.go GEO

Contact: webmaster@%s
Policy: %s/ai-content-policy
Version: 1.0
`, cfg.SiteName, extractDomain(cfg.SiteURL), cfg.SiteURL)
}

// GenerateLLMtxt builds the llms.txt markdown site index
func GenerateLLMtxt(routes []router.Route, pages map[string]*PageGEO, cfg *config.GEOConfig) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n", cfg.SiteName))
	sb.WriteString(fmt.Sprintf("> %s\n\n", cfg.SiteURL))
	sb.WriteString("## Pages\n\n")

	for _, r := range routes {
		if r.Is404 {
			continue
		}
		title := r.Pattern
		desc := ""
		if pg, ok := pages[r.Pattern]; ok && pg != nil {
			if pg.Title != "" {
				title = pg.Title
			}
			desc = pg.Description
			if len(desc) > cfg.LLMTxt.MaxChars {
				desc = desc[:cfg.LLMTxt.MaxChars]
			}
		}
		sb.WriteString(fmt.Sprintf("- [%s](%s%s)", title, cfg.SiteURL, r.Pattern))
		if desc != "" {
			sb.WriteString(fmt.Sprintf(" — %s", desc))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func extractDomain(url string) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	if idx := strings.Index(url, "/"); idx != -1 {
		url = url[:idx]
	}
	return url
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
