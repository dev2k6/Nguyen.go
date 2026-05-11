package optimizer

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// OptimizeOptions configures image optimization behavior
type OptimizeOptions struct {
	Width   int           // Target width in pixels
	Quality int           // 1-100, quality level
	Formats []ImageFormat // Output formats (e.g. ["webp", "jpeg"])
	Sizes   []int         // Responsive widths for srcset (e.g. [640, 1080, 1920])
}

// OptimizedImage holds the output of image optimization
type OptimizedImage struct {
	Src         string         // Original source path
	Width       int            // Original image width
	Height      int            // Original image height
	SrcSet      map[int]string // Width → URL path (e.g. {640: "/_nguyen/image?src=...&w=640"})
	BlurDataURL string         // Base64 data URI for LQIP
	CacheKey    string         // Hash key for the primary optimized version
	CachePath   string         // Disk path to the cached optimized file
	Format      ImageFormat    // Primary format used
}

// Service orchestrates image optimization with disk caching
type Service struct {
	CacheDir  string
	PublicDir string // for resolving source file paths
}

// NewService creates a new image optimizer service
func NewService(cacheDir, publicDir string) *Service {
	os.MkdirAll(cacheDir, 0755)
	return &Service{CacheDir: cacheDir, PublicDir: publicDir}
}

// Optimize processes an image: resize, convert format, cache, generate blur placeholder.
// Returns metadata suitable for injecting into HTML srcset and blur attributes.
func (s *Service) Optimize(srcPath string, opts OptimizeOptions) (*OptimizedImage, error) {
	fullPath := srcPath
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(s.PublicDir, srcPath)
	}

	// Read and decode source
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("optimize: cannot read %s: %w", fullPath, err)
	}

	img, err := DecodeImage(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("optimize: cannot decode %s: %w", fullPath, err)
	}

	bounds := img.Bounds()
	origW := bounds.Dx()
	origH := bounds.Dy()

	// Select primary format (first in list or JPEG)
	format := FormatJPEG
	if len(opts.Formats) > 0 {
		format = opts.Formats[0]
	}

	quality := opts.Quality
	if quality == 0 {
		quality = 80
	}

	width := opts.Width
	if width == 0 || width > origW {
		width = origW
	}

	// Compute hash and check cache
	hash := ComputeHash(fullPath, width, quality, format)
	cachePath := CachePath(s.CacheDir, hash, format)

	// Check if already cached
	if _, err := os.Stat(cachePath); err == nil {
		// Already cached — generate metadata from cache
		blur, _ := GenerateBlurPlaceholder(img, 10)
		return &OptimizedImage{
			Src:         srcPath,
			Width:       origW,
			Height:      origH,
			SrcSet:      s.buildSrcSet(srcPath, opts.Sizes, opts.Quality, format),
			BlurDataURL: blur,
			CacheKey:    hash,
			CachePath:   cachePath,
			Format:      format,
		}, nil
	}

	// Resize
	resized := Resize(img, width)
	newW := resized.Bounds().Dx()
	newH := resized.Bounds().Dy()

	// Encode
	encoded, err := EncodeImage(resized, format, quality)
	if err != nil {
		return nil, fmt.Errorf("optimize: encode failed: %w", err)
	}

	// Write to disk cache
	if err := os.WriteFile(cachePath, encoded, 0644); err != nil {
		return nil, fmt.Errorf("optimize: cache write failed: %w", err)
	}

	// Generate responsive sizes concurrently (cache each size)
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	for _, w := range opts.Sizes {
		if w <= 0 || w >= origW {
			continue
		}
		w := w
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			sh := ComputeHash(fullPath, w, quality, format)
			sp := CachePath(s.CacheDir, sh, format)
			if _, err := os.Stat(sp); err == nil {
				return // already cached
			}
			ri := Resize(img, w)
			ed, eErr := EncodeImage(ri, format, quality)
			if eErr != nil {
				return
			}
			os.WriteFile(sp, ed, 0644)
		}()
	}
	wg.Wait()

	// Generate blur placeholder
	blur, err := GenerateBlurPlaceholder(img, 10)
	if err != nil {
		blur = ""
	}

	return &OptimizedImage{
		Src:         srcPath,
		Width:       newW,
		Height:      newH,
		SrcSet:      s.buildSrcSet(srcPath, opts.Sizes, quality, format),
		BlurDataURL: blur,
		CacheKey:    hash,
		CachePath:   cachePath,
		Format:      format,
	}, nil
}

// buildSrcSet generates a map of width → URL for responsive images
func (s *Service) buildSrcSet(src string, sizes []int, quality int, format ImageFormat) map[int]string {
	result := make(map[int]string)
	for _, w := range sizes {
		if w <= 0 {
			continue
		}
		result[w] = fmt.Sprintf("/_nguyen/image?src=%s&w=%d&q=%d&f=%s", src, w, quality, format)
	}
	return result
}
