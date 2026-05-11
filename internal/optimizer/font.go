package optimizer

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var fontHTTPClient = &http.Client{Timeout: 30 * time.Second}

// OptimizedFont holds the inline-optimized font data
type OptimizedFont struct {
	Family  string
	CSS     string // Complete @font-face block with base64 data URI
	Weight  string
	Style   string
	Display string
}

// FontOptimizer downloads, caches, and encodes fonts for inline use
type FontOptimizer struct {
	CacheDir string
}

// NewFontOptimizer creates a new font optimizer
func NewFontOptimizer(cacheDir string) *FontOptimizer {
	os.MkdirAll(cacheDir, 0755)
	return &FontOptimizer{CacheDir: cacheDir}
}

// OptimizeGoogleFont downloads a Google Font CSS file, extracts the font URL,
// downloads the actual font file, and base64-encodes it for inline embedding.
func (f *FontOptimizer) OptimizeGoogleFont(fontCSSURL, family, weight, style, display string) (*OptimizedFont, error) {
	// Fetch Google Fonts CSS
	resp, err := fontHTTPClient.Get(fontCSSURL)
	if err != nil {
		return nil, fmt.Errorf("font: download failed: %w", err)
	}
	defer resp.Body.Close()

	cssData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("font: read failed: %w", err)
	}

	cssStr := string(cssData)

	// Extract font file URL from CSS
	// @font-face { src: url(https://fonts.gstatic.com/...) format('woff2'); }
	fontURL := extractFontURL(cssStr)
	if fontURL == "" {
		return nil, fmt.Errorf("font: could not extract font URL from CSS")
	}

	// Compute cache key
	hash := sha256.Sum256([]byte(fontURL))
	cacheKey := fmt.Sprintf("%x", hash)[:12]
	cachePath := filepath.Join(f.CacheDir, cacheKey+".woff2")

	// Check cache
	var fontData []byte
	if cached, err := os.ReadFile(cachePath); err == nil {
		fontData = cached
	} else {
		// Download font file
		fResp, fErr := fontHTTPClient.Get(fontURL)
		if fErr != nil {
			return nil, fmt.Errorf("font: download font file failed: %w", fErr)
		}
		defer fResp.Body.Close()

		fontData, err = io.ReadAll(fResp.Body)
		if err != nil {
			return nil, fmt.Errorf("font: read font file failed: %w", err)
		}

		// Cache to disk
		os.WriteFile(cachePath, fontData, 0644)
	}

	// Base64 encode
	b64 := base64.StdEncoding.EncodeToString(fontData)

	// Build @font-face CSS
	format := "woff2"
	if strings.HasSuffix(fontURL, ".ttf") {
		format = "truetype"
	} else if strings.HasSuffix(fontURL, ".woff") {
		format = "woff"
	}

	css := fmt.Sprintf(`@font-face {
	font-family: '%s';
	font-style: %s;
	font-weight: %s;
	font-display: %s;
	src: url(data:font/%s;base64,%s) format('%s');
}`, family, style, weight, display, format, b64, format)

	return &OptimizedFont{
		Family:  family,
		CSS:     css,
		Weight:  weight,
		Style:   style,
		Display: display,
	}, nil
}

// BuildFontStyleTag wraps font CSS in a <style> tag for injection into <head>
func BuildFontStyleTag(css string) string {
	if css == "" {
		return ""
	}
	return fmt.Sprintf("<style>\n%s\n</style>", css)
}

// extractFontURL extracts the first src: url(...) from a CSS string
func extractFontURL(css string) string {
	idx := strings.Index(css, "url(")
	if idx == -1 {
		return ""
	}
	start := idx + 4
	end := strings.Index(css[start:], ")")
	if end == -1 {
		return ""
	}
	return css[start : start+end]
}
