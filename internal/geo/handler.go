package geo

import (
	"fmt"
	"sync"
	"time"

	"github.com/dev2k6/Nguyen.go/internal/config"
	"github.com/dev2k6/Nguyen.go/internal/router"

	"github.com/gofiber/fiber/v2"
)

// Handler serves GEO routes with caching
type Handler struct {
	cfg             *config.GEOConfig
	routes          []router.Route
	sitemapCache    string
	sitemapCachedAt time.Time
	robotsCache     string
	llmCache        string
	llmCachedAt     time.Time
	aiBotCache      string
	mu              sync.RWMutex
}

// NewHandler creates a GEO route handler
func NewHandler(cfg *config.GEOConfig, routes []router.Route) *Handler {
	return &Handler{
		cfg:    cfg,
		routes: routes,
	}
}

// Sitemap serves /sitemap.xml
func (h *Handler) Sitemap(c *fiber.Ctx) error {
	if !h.cfg.Enabled || !h.cfg.Sitemap.Enabled {
		return c.Status(404).SendString("Not Found")
	}

	h.mu.RLock()
	cacheTTL := time.Duration(h.cfg.CacheTTL) * time.Second
	if h.sitemapCache != "" && time.Since(h.sitemapCachedAt) < cacheTTL {
		xml := h.sitemapCache
		h.mu.RUnlock()
		c.Set("Content-Type", "application/xml; charset=utf-8")
		c.Set("Cache-Control", "public, max-age="+fmtDuration(cacheTTL))
		return c.SendString(xml)
	}
	h.mu.RUnlock()

	xml := GenerateSitemapXML(h.routes, h.cfg)

	h.mu.Lock()
	h.sitemapCache = xml
	h.sitemapCachedAt = time.Now()
	h.mu.Unlock()

	c.Set("Content-Type", "application/xml; charset=utf-8")
	c.Set("Cache-Control", "public, max-age="+fmtDuration(cacheTTL))
	return c.SendString(xml)
}

// Robots serves /robots.txt
func (h *Handler) Robots(c *fiber.Ctx) error {
	if !h.cfg.Enabled || !h.cfg.Robots.Enabled {
		c.Set("Content-Type", "text/plain")
		return c.SendString("User-agent: *\nAllow: /\n")
	}

	h.mu.RLock()
	if h.robotsCache != "" {
		txt := h.robotsCache
		h.mu.RUnlock()
		c.Set("Content-Type", "text/plain; charset=utf-8")
		return c.SendString(txt)
	}
	h.mu.RUnlock()

	txt := GenerateRobotsTxt(h.cfg)

	h.mu.Lock()
	h.robotsCache = txt
	h.mu.Unlock()

	c.Set("Content-Type", "text/plain; charset=utf-8")
	return c.SendString(txt)
}

// LLMTxt serves /llms.txt
func (h *Handler) LLMTxt(c *fiber.Ctx) error {
	if !h.cfg.Enabled || !h.cfg.LLMTxt.Enabled {
		return c.Status(404).SendString("Not Found")
	}

	h.mu.RLock()
	cacheTTL := time.Duration(h.cfg.CacheTTL) * time.Second
	if h.llmCache != "" && time.Since(h.llmCachedAt) < cacheTTL {
		txt := h.llmCache
		h.mu.RUnlock()
		c.Set("Content-Type", "text/plain; charset=utf-8")
		c.Set("Cache-Control", "public, max-age="+fmtDuration(cacheTTL))
		return c.SendString(txt)
	}
	h.mu.RUnlock()

	txt := GenerateLLMtxt(h.routes, nil, h.cfg)

	h.mu.Lock()
	h.llmCache = txt
	h.llmCachedAt = time.Now()
	h.mu.Unlock()

	c.Set("Content-Type", "text/plain; charset=utf-8")
	c.Set("Cache-Control", "public, max-age="+fmtDuration(cacheTTL))
	return c.SendString(txt)
}

// AIBot serves /.well-known/ai-bot.txt
func (h *Handler) AIBot(c *fiber.Ctx) error {
	if !h.cfg.Enabled {
		return c.Status(404).SendString("Not Found")
	}

	h.mu.RLock()
	if h.aiBotCache != "" {
		txt := h.aiBotCache
		h.mu.RUnlock()
		c.Set("Content-Type", "text/plain; charset=utf-8")
		return c.SendString(txt)
	}
	h.mu.RUnlock()

	txt := GenerateAIBotTxt(h.cfg)

	h.mu.Lock()
	h.aiBotCache = txt
	h.mu.Unlock()

	c.Set("Content-Type", "text/plain; charset=utf-8")
	return c.SendString(txt)
}

func fmtDuration(d time.Duration) string {
	return fmt.Sprintf("%.0f", d.Seconds())
}
