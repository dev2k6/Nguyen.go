package optimizer

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
)

// ImageFormat represents a supported output format
type ImageFormat string

const (
	FormatWebP ImageFormat = "webp"
	FormatJPEG ImageFormat = "jpeg"
	FormatPNG  ImageFormat = "png"
	FormatAVIF ImageFormat = "avif"
)

// Resize scales an image to the target width while preserving aspect ratio.
// Uses BiLinear interpolation for speed-quality balance.
func Resize(src image.Image, targetWidth int) image.Image {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	if targetWidth >= srcW {
		return src
	}

	ratio := float64(targetWidth) / float64(srcW)
	targetHeight := int(float64(srcH) * ratio)

	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}

// EncodeJPEG encodes an image to JPEG format with given quality (1-100).
func EncodeJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// EncodePNG encodes an image to PNG format.
func EncodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// EncodeImage encodes an image to the specified format.
// WebP and AVIF fallback to JPEG if Go stdlib encoders unavailable.
func EncodeImage(img image.Image, format ImageFormat, quality int) ([]byte, error) {
	switch format {
	case FormatJPEG:
		return EncodeJPEG(img, quality)
	case FormatPNG:
		return EncodePNG(img)
	case FormatWebP, FormatAVIF:
		// Pure Go WebP/AVIF encoding requires external packages.
		// Fallback to JPEG at high quality for now.
		// TODO: add golang.org/x/image/webp encoder when available
		return EncodeJPEG(img, min(quality+10, 100))
	default:
		return EncodeJPEG(img, quality)
	}
}

// EncodeFormat returns the MIME type for a format
func EncodeFormat(f ImageFormat) string {
	switch f {
	case FormatWebP:
		return "image/webp"
	case FormatAVIF:
		return "image/avif"
	case FormatPNG:
		return "image/png"
	default:
		return "image/jpeg"
	}
}

// EncodeFormatExt returns the file extension for a format
func EncodeFormatExt(f ImageFormat) string {
	switch f {
	case FormatWebP:
		return ".webp"
	case FormatAVIF:
		return ".avif"
	case FormatPNG:
		return ".png"
	default:
		return ".jpg"
	}
}

// maxImageDimension is the largest width or height accepted before
// full decode. A 8192×8192 RGBA image uses 256 MB — large enough for
// any legitimate use case while blocking decompression bombs.
const maxImageDimension = 8192

// DecodeImage reads and decodes an image from a reader.
// Supports JPEG and PNG. Returns an error if either dimension exceeds
// maxImageDimension to prevent decompression bomb DoS attacks.
func DecodeImage(r io.Reader) (image.Image, error) {
	// Peek at the config (dimensions only) before full decode.
	// image.DecodeConfig reads only the header — cheap and safe.
	buf := &peekReader{}
	cfg, format, err := image.DecodeConfig(io.TeeReader(r, buf))
	if err != nil {
		return nil, fmt.Errorf("image: cannot read header: %w", err)
	}
	if cfg.Width > maxImageDimension || cfg.Height > maxImageDimension {
		return nil, fmt.Errorf("image: dimensions %dx%d exceed maximum %d (format: %s)",
			cfg.Width, cfg.Height, maxImageDimension, format)
	}
	// Replay the already-read bytes followed by the rest of the stream.
	img, _, err := image.Decode(io.MultiReader(buf, r))
	return img, err
}

// peekReader buffers bytes written to it so they can be replayed.
type peekReader struct {
	buf bytes.Buffer
}

func (p *peekReader) Write(b []byte) (int, error) { return p.buf.Write(b) }
func (p *peekReader) Read(b []byte) (int, error)  { return p.buf.Read(b) }

// GenerateBlurPlaceholder creates a low-quality blur placeholder (LQIP).
// Downsamples the source image to a tiny size, encodes as JPEG at low quality,
// and returns a base64 data URI string suitable for inline <img src="...">.
func GenerateBlurPlaceholder(src image.Image, size int) (string, error) {
	// Resize to tiny thumbnail
	thumb := Resize(src, size)

	// Encode as low quality JPEG
	data, err := EncodeJPEG(thumb, 30)
	if err != nil {
		return "", fmt.Errorf("blur placeholder: %w", err)
	}

	// Base64 encode
	b64 := base64.StdEncoding.EncodeToString(data)
	return "data:image/jpeg;base64," + b64, nil
}

// ComputeHash generates a content-based hash for cache invalidation.
func ComputeHash(srcPath string, width, quality int, format ImageFormat) string {
	input := fmt.Sprintf("%s-%d-%d-%s", srcPath, width, quality, format)
	h := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", h)[:12]
}

// CachePath returns the disk cache path for an optimized image.
func CachePath(cacheDir, hash string, format ImageFormat) string {
	ext := EncodeFormatExt(format)
	return filepath.Join(cacheDir, hash+ext)
}

// LoadCachedImage reads a previously optimized image from disk cache.
func LoadCachedImage(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
