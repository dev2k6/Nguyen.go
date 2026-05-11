# Image Optimization

Nguyen.go includes a built-in image optimization endpoint that converts, resizes, and caches images on-the-fly.

## Endpoint

```
GET /_nguyen/image?src=<path>&w=<width>&f=<format>&q=<quality>
```

### Parameters

| Param | Description | Default |
|-------|-------------|---------|
| `src` | Image path relative to public dir | required |
| `w` | Output width in pixels | original |
| `f` | Output format (`webp`, `avif`, `jpeg`, `png`) | from config |
| `q` | Quality (1-100) | from config |

### Example

```html
<img src="/_nguyen/image?src=/photos/hero.jpg&w=1080&f=webp&q=80" alt="Hero">
```

## Configuration

```yaml
images:
  formats: [webp, avif]           # Preferred output formats
  sizes: [640, 750, 1080, 1920]   # Responsive breakpoints
  quality: 80                      # Default quality
  cache_ttl: 86400                 # Cache duration (seconds)
  cache_dir: .nguyen/cache/images  # Disk cache location
  public_dir: public              # Source images directory
  blur_size: 10                    # Blur placeholder dimensions
```

## Responsive Images

Generate srcset for responsive images:

```html
<img
    src="/_nguyen/image?src=/photos/hero.jpg&w=1080&f=webp"
    srcset="
        /_nguyen/image?src=/photos/hero.jpg&w=640&f=webp 640w,
        /_nguyen/image?src=/photos/hero.jpg&w=750&f=webp 750w,
        /_nguyen/image?src=/photos/hero.jpg&w=1080&f=webp 1080w,
        /_nguyen/image?src=/photos/hero.jpg&w=1920&f=webp 1920w
    "
    sizes="(max-width: 768px) 100vw, 1080px"
    alt="Hero image"
/>
```

## Supported Formats

| Format | Extension | Notes |
|--------|-----------|-------|
| WebP | `.webp` | Best compression/quality ratio |
| AVIF | `.avif` | Smallest files, newer browser support |
| JPEG | `.jpeg` | Universal compatibility |
| PNG | `.png` | Lossless, transparency support |

## Caching

- Images are cached to disk after first conversion
- Cache key: `{src}_{width}_{format}_{quality}`
- TTL configurable via `cache_ttl` (default 24 hours)
- Cache directory: `.nguyen/cache/images/`

## Blur Placeholders

Generate tiny blur placeholders for progressive loading:

```
GET /_nguyen/image?src=/photos/hero.jpg&w=10&q=20
```

Use as a base64-encoded inline placeholder while the full image loads.

## Security

- Path traversal protection: `src` is validated against the public directory
- Only files within `public_dir` can be served
- Maximum output dimensions are bounded by configured `sizes`

## Performance

- First request: image is processed and cached (may take 100-500ms)
- Subsequent requests: served from disk cache (< 5ms)
- Cache headers: `Cache-Control: public, max-age={cache_ttl}`
- ETag support for conditional requests
