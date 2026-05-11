# GEO (Generative Engine Optimization)

Nguyen.go includes built-in support for making your site discoverable by both traditional search engines and AI systems.

## Overview

GEO goes beyond traditional SEO by optimizing for:
- Search engine crawlers (Google, Bing)
- AI assistants (ChatGPT, Claude, Perplexity)
- Social media previews (Open Graph, Twitter Cards)
- Structured data consumers (Google Rich Results)

## Configuration

Enable GEO in `config/nguyen.config.yml`:

```yaml
geo:
  enabled: true
  site_name: "My App"
  site_url: "https://example.com"
  organization_name: "My Organization"
  organization_url: "https://example.com"
  same_as:
    - "https://github.com/my-org"
    - "https://twitter.com/my-org"
  author_type: Organization    # Organization | Person
  default_page_type: Article
  sitemap:
    enabled: true
    default_changefreq: weekly
    default_priority: 0.7
  robots:
    enabled: true
    allow_ai_bots: false
  llms_txt:
    enabled: true
    max_chars_per_page: 5000
  cache_ttl: 3600
```

## Auto-Generated Endpoints

### `/sitemap.xml`

XML sitemap listing all discoverable routes with metadata:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://example.com/</loc>
    <changefreq>weekly</changefreq>
    <priority>0.7</priority>
  </url>
  <url>
    <loc>https://example.com/about</loc>
    <changefreq>weekly</changefreq>
    <priority>0.7</priority>
  </url>
</urlset>
```

### `/robots.txt`

Crawler directives:

```
User-agent: *
Allow: /
Sitemap: https://example.com/sitemap.xml

# AI Bots
User-agent: GPTBot
Disallow: /

User-agent: ChatGPT-User
Disallow: /

User-agent: anthropic-ai
Disallow: /
```

Set `allow_ai_bots: true` to permit AI crawlers.

### `/llms.txt`

LLM-friendly plain text summary of your site content:

```
# My App

> A full-stack Go web framework

## Pages

### Home (/)
File-based routing, islands architecture, React-style hooks...

### About (/about)
Learn more about the project...

### Blog (/blog)
Latest articles and updates...
```

Content is truncated per page based on `max_chars_per_page`.

### `/.well-known/ai-bot.txt`

AI bot permissions file following the emerging standard:

```
User-agent: *
Allow: /
```

## Per-Page GEO Metadata

Define structured data in page frontmatter:

```nguyen
---
export const geo = {
    pageType: "Article",
    datePublished: "2026-01-15T00:00:00Z",
    dateModified: "2026-05-10T00:00:00Z",
    author: "Thái Nguyên",
    speakable: [".main-content"],
    breadcrumbs: [
        {name: "Home", url: "/"},
        {name: "Blog", url: "/blog"},
        {name: "This Post", url: "/blog/this-post"},
    ],
    faqs: [
        {question: "What is Nguyen.go?", answer: "A full-stack Go web framework."},
    ],
    howToSteps: [
        {name: "Install", text: "Run go install"},
        {name: "Create", text: "Run nguyen create my-app"},
    ],
    tags: ["go", "webassembly"],
    images: ["/og-image.png"],
    sameAs: ["https://github.com/dev2k6/Nguyen.go"],
}
---
```

### Supported Fields

| Field | Type | Description |
|-------|------|-------------|
| `pageType` | string | Schema.org type (Article, HowTo, FAQPage, etc.) |
| `datePublished` | string | ISO 8601 publish date |
| `dateModified` | string | ISO 8601 last modified date |
| `author` | string | Author name |
| `speakable` | []string | CSS selectors for speakable content |
| `breadcrumbs` | []object | Navigation breadcrumb trail |
| `faqs` | []object | FAQ question/answer pairs |
| `howToSteps` | []object | Step-by-step instructions |
| `tags` | []string | Content tags/keywords |
| `images` | []string | Associated image URLs |
| `sameAs` | []string | Related URLs (social profiles) |

## JSON-LD Output

GEO metadata generates JSON-LD structured data in the HTML:

```html
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "Article",
  "headline": "Blog Post Title",
  "datePublished": "2026-01-15T00:00:00Z",
  "dateModified": "2026-05-10T00:00:00Z",
  "author": {
    "@type": "Person",
    "name": "Thái Nguyên"
  },
  "breadcrumb": {
    "@type": "BreadcrumbList",
    "itemListElement": [...]
  }
}
</script>
```

## Open Graph Tags

Metadata from frontmatter generates OG tags:

```html
<meta property="og:title" content="Blog Post Title">
<meta property="og:description" content="Page description">
<meta property="og:image" content="/og-image.png">
<meta property="og:type" content="article">
<meta property="article:published_time" content="2026-01-15T00:00:00Z">
<meta property="article:modified_time" content="2026-05-10T00:00:00Z">
```

## FAQ Schema

Define FAQ pairs for Google Rich Results:

```nguyen
---
export const geo = {
    pageType: "FAQPage",
    faqs: [
        {question: "How do I install?", answer: "Run go install github.com/dev2k6/Nguyen.go/cmd/nguyen@latest"},
        {question: "Does it need Node.js?", answer: "No, Nguyen.go is pure Go."},
        {question: "What rendering modes are supported?", answer: "SSR, ISR, and CSR."},
    ],
}
---
```

## HowTo Schema

Define step-by-step instructions:

```nguyen
---
export const geo = {
    pageType: "HowTo",
    howToSteps: [
        {name: "Install the CLI", text: "Run go install github.com/dev2k6/Nguyen.go/cmd/nguyen@latest"},
        {name: "Create a project", text: "Run nguyen create my-app"},
        {name: "Start developing", text: "Run nguyen dev to start the dev server"},
    ],
}
---
```

## Caching

GEO responses are cached based on `cache_ttl` (default 3600 seconds). The cache is invalidated when routes change.

## Static Export

When using `nguyen export`, GEO files are generated as static files:

```bash
nguyen export --output dist
# Creates:
#   dist/sitemap.xml
#   dist/robots.txt
#   dist/llms.txt
```
