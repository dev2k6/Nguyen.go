package geo

import (
	"encoding/json"
	"fmt"

	"github.com/dev2k6/Nguyen.go/internal/config"
)

// ArticleSchema builds JSON-LD for Article types
func ArticleSchema(p *PageGEO, cfg *config.GEOConfig) map[string]interface{} {
	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    p.PageType,
	}

	if p.Title != "" {
		schema["headline"] = p.Title
	}
	if p.Description != "" {
		schema["description"] = p.Description
	}
	if p.DatePublished != "" {
		schema["datePublished"] = p.DatePublished
	}
	if p.DateModified != "" {
		schema["dateModified"] = p.DateModified
	}
	if len(p.Images) > 0 {
		schema["image"] = p.Images
	}

	// Author
	if p.Author != "" {
		if cfg.AuthorType == "Person" {
			schema["author"] = map[string]interface{}{
				"@type": "Person",
				"name":  p.Author,
			}
		} else {
			schema["author"] = map[string]interface{}{
				"@type": "Organization",
				"name":  p.Author,
			}
		}
	} else {
		// Default author from config
		if cfg.AuthorType == "Person" {
			schema["author"] = map[string]interface{}{
				"@type": "Person",
				"name":  cfg.OrganizationName,
			}
		} else {
			schema["author"] = map[string]interface{}{
				"@type": "Organization",
				"name":  cfg.OrganizationName,
			}
		}
	}

	// Publisher
	schema["publisher"] = map[string]interface{}{
		"@type": "Organization",
		"name":  cfg.OrganizationName,
		"url":   cfg.OrganizationURL,
	}

	return schema
}

// FAQPageSchema builds JSON-LD for FAQ page
func FAQPageSchema(p *PageGEO, cfg *config.GEOConfig) map[string]interface{} {
	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "FAQPage",
	}

	if p.Title != "" {
		schema["headline"] = p.Title
	}
	if p.Description != "" {
		schema["description"] = p.Description
	}

	if len(p.Faqs) > 0 {
		var entities []map[string]interface{}
		for _, faq := range p.Faqs {
			entities = append(entities, map[string]interface{}{
				"@type": "Question",
				"name":  faq.Question,
				"acceptedAnswer": map[string]interface{}{
					"@type": "Answer",
					"text":  faq.Answer,
				},
			})
		}
		schema["mainEntity"] = entities
	}

	return schema
}

// HowToSchema builds JSON-LD for HowTo guides
func HowToSchema(p *PageGEO, cfg *config.GEOConfig) map[string]interface{} {
	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "HowTo",
	}

	if p.Title != "" {
		schema["name"] = p.Title
	}
	if p.Description != "" {
		schema["description"] = p.Description
	}
	if p.DatePublished != "" {
		schema["datePublished"] = p.DatePublished
	}

	// Author
	if p.Author != "" {
		schema["author"] = map[string]interface{}{
			"@type": "Person",
			"name":  p.Author,
		}
	}

	// Steps
	if len(p.HowToSteps) > 0 {
		var steps []map[string]interface{}
		for i, step := range p.HowToSteps {
			stepName := step.Name
			if stepName == "" {
				stepName = fmt.Sprintf("Step %d", i+1)
			}
			steps = append(steps, map[string]interface{}{
				"@type": "HowToStep",
				"name":  stepName,
				"text":  step.Text,
			})
		}
		schema["step"] = steps
	}

	return schema
}

// BreadcrumbListSchema builds JSON-LD for breadcrumb navigation
func BreadcrumbListSchema(p *PageGEO) string {
	var items []map[string]interface{}
	for i, bc := range p.Breadcrumbs {
		items = append(items, map[string]interface{}{
			"@type":    "ListItem",
			"position": i + 1,
			"name":     bc.Name,
			"item":     bc.URL,
		})
	}

	schema := map[string]interface{}{
		"@context":        "https://schema.org",
		"@type":           "BreadcrumbList",
		"itemListElement": items,
	}

	b, _ := json.Marshal(schema)
	return string(b)
}

// OrganizationSchema builds JSON-LD for an Organization entity
func OrganizationSchema(cfg *config.GEOConfig) string {
	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "Organization",
		"name":     cfg.OrganizationName,
		"url":      cfg.OrganizationURL,
	}

	if cfg.SiteName != "" {
		schema["name"] = cfg.SiteName
	}

	if len(cfg.SameAs) > 0 {
		schema["sameAs"] = cfg.SameAs
	}

	b, _ := json.Marshal(schema)
	return string(b)
}

// WebSiteSchema builds JSON-LD for a WebSite entity with SearchAction
func WebSiteSchema(cfg *config.GEOConfig) string {
	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "WebSite",
		"name":     cfg.SiteName,
		"url":      cfg.SiteURL,
	}

	// SearchAction for AI feature
	schema["potentialAction"] = map[string]interface{}{
		"@type": "SearchAction",
		"target": map[string]string{
			"@type":       "EntryPoint",
			"urlTemplate": cfg.SiteURL + "/search?q={search_term_string}",
		},
		"query-input": "required name=search_term_string",
	}

	b, _ := json.Marshal(schema)
	return string(b)
}

// WebPageSchema builds a generic WebPage JSON-LD
func WebPageSchema(p *PageGEO, cfg *config.GEOConfig) map[string]interface{} {
	schema := map[string]interface{}{
		"@context": "https://schema.org",
		"@type":    "WebPage",
	}

	if p.Title != "" {
		schema["name"] = p.Title
	}
	if p.Description != "" {
		schema["description"] = p.Description
	}
	if p.DatePublished != "" {
		schema["datePublished"] = p.DatePublished
	}
	if p.DateModified != "" {
		schema["dateModified"] = p.DateModified
	}

	// About tags
	if len(p.Tags) > 0 {
		var about []map[string]interface{}
		for _, tag := range p.Tags {
			about = append(about, map[string]interface{}{
				"@type": "Thing",
				"name":  tag,
			})
		}
		schema["about"] = about
	}

	return schema
}

// SpeakableSpecificationSchema builds JSON-LD for AI-readable speakable content
func SpeakableSpecificationSchema(selectors []string) string {
	schema := map[string]interface{}{
		"@context":    "https://schema.org",
		"@type":       "SpeakableSpecification",
		"cssSelector": selectors,
	}

	b, _ := json.Marshal(schema)
	return string(b)
}
