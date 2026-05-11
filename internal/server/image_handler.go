package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dev2k6/Nguyen.go/internal/config"
	"github.com/dev2k6/Nguyen.go/internal/optimizer"

	"github.com/gofiber/fiber/v2"
)

// ImageHandler creates a Fiber handler for the /_nguyen/image endpoint.
// Query params: src (required), w (width), q (quality), f (format)
func ImageHandler(cfg *config.NguyenConfig) fiber.Handler {
	imgCfg := cfg.Images
	cacheDir := imgCfg.CacheDir
	if cacheDir == "" {
		cacheDir = ".nguyen/cache/images"
	}
	publicDir := imgCfg.PublicDir
	if publicDir == "" {
		publicDir = "public"
	}

	svc := optimizer.NewService(cacheDir, publicDir)
	absPublicDir, _ := filepath.Abs(publicDir)

	return func(c *fiber.Ctx) error {
		src := c.Query("src")
		if src == "" {
			return c.Status(400).JSON(fiber.Map{"error": "missing src parameter"})
		}

		// Prevent path traversal attacks
		src = filepath.Clean(src)
		if strings.Contains(src, "..") || filepath.IsAbs(src) {
			return c.Status(400).JSON(fiber.Map{"error": "invalid src parameter"})
		}
		if strings.HasPrefix(src, "/") || strings.HasPrefix(src, "\\") {
			return c.Status(400).JSON(fiber.Map{"error": "invalid src parameter"})
		}

		// Defense-in-depth: ensure the joined path stays under publicDir even
		// after symlink resolution. Protects against symlink escapes and
		// platform-specific path quirks.
		resolved, err := filepath.Abs(filepath.Join(publicDir, src))
		if err != nil || (absPublicDir != "" && !strings.HasPrefix(resolved, absPublicDir+string(filepath.Separator)) && resolved != absPublicDir) {
			return c.Status(400).JSON(fiber.Map{"error": "invalid src parameter"})
		}

		width, _ := strconv.Atoi(c.Query("w", "0"))
		quality, _ := strconv.Atoi(c.Query("q", fmt.Sprint(imgCfg.Quality)))

		formatStr := c.Query("f", "jpeg")
		format := optimizer.FormatJPEG
		switch formatStr {
		case "webp":
			format = optimizer.FormatWebP
		case "jpeg", "jpg":
			format = optimizer.FormatJPEG
		case "png":
			format = optimizer.FormatPNG
		case "avif":
			format = optimizer.FormatAVIF
		}

		opts := optimizer.OptimizeOptions{
			Width:   width,
			Quality: quality,
			Formats: []optimizer.ImageFormat{format},
			Sizes:   imgCfg.Sizes,
		}

		img, err := svc.Optimize(src, opts)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "image not found"})
		}

		// Serve from disk cache
		data, err := os.ReadFile(img.CachePath)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "cache read failed"})
		}

		c.Set("Content-Type", optimizer.EncodeFormat(format))
		c.Set("Cache-Control", "public, max-age=31536000, immutable")
		return c.Send(data)
	}
}
