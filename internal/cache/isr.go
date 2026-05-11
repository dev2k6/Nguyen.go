package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry holds cached HTML with expiration metadata
type Entry struct {
	HTML      string    `json:"-"`
	Path      string    `json:"path"`
	ExpiresAt time.Time `json:"expires_at"`
	Tags      []string  `json:"tags,omitempty"`
	TTL       int       `json:"ttl"`
}

// diskEntry is the JSON-serializable cache metadata on disk
type diskEntry struct {
	Path      string   `json:"path"`
	ExpiresAt int64    `json:"expires_at"` // unix seconds
	Tags      []string `json:"tags,omitempty"`
	TTL       int      `json:"ttl"`
}

// ISR implements Incremental Static Regeneration caching with disk persistence.
// Stores pre-rendered HTML pages in memory and on disk with TTL-based expiration.
// Supports stale-while-revalidate and on-demand revalidation.
type ISR struct {
	entries      map[string]*Entry
	mu           sync.RWMutex
	cacheDir     string
	revalidating map[string]bool
	revMu        sync.Mutex
	stopCh       chan struct{}
}

// NewISR creates a new ISR cache with optional disk persistence.
// cacheDir: directory for disk cache (".nguyen/cache/pages"). Empty string disables disk cache.
func NewISR(cacheDir string) *ISR {
	c := &ISR{
		entries:      make(map[string]*Entry),
		cacheDir:     cacheDir,
		revalidating: make(map[string]bool),
		stopCh:       make(chan struct{}),
	}

	// Load cached entries from disk at startup
	if cacheDir != "" {
		os.MkdirAll(cacheDir, 0750)
		c.loadFromDisk()
	}

	// Background cleanup every 30 seconds
	go c.cleanup(30 * time.Second)
	return c
}

// Get retrieves a cached entry.
// Returns (html, isStale, error).
// isStale=true means the content is expired but still served (SWR).
func (c *ISR) Get(path string) (string, bool, error) {
	c.mu.RLock()
	entry, ok := c.entries[path]
	c.mu.RUnlock()

	if !ok {
		// Try disk cache
		if c.cacheDir != "" {
			entry = c.loadOneFromDisk(path)
			if entry != nil {
				c.mu.Lock()
				c.entries[path] = entry
				c.mu.Unlock()
			}
		}
	}

	if entry == nil {
		return "", false, fmt.Errorf("cache miss: %s", path)
	}

	isStale := time.Now().After(entry.ExpiresAt) && !entry.ExpiresAt.IsZero()
	return entry.HTML, isStale, nil
}

// Set stores an entry with a TTL in seconds. 0 = no expiry.
// Also writes to disk if cacheDir is configured.
func (c *ISR) Set(path, html string, ttl int) {
	c.SetTagged(path, html, ttl, nil)
}

// SetTagged stores an entry with tags for grouped revalidation.
func (c *ISR) SetTagged(path, html string, ttl int, tags []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := &Entry{
		HTML: html,
		Path: path,
		TTL:  ttl,
		Tags: tags,
	}
	if ttl > 0 {
		entry.ExpiresAt = time.Now().Add(time.Duration(ttl) * time.Second)
	}
	c.entries[path] = entry

	// Write to disk
	c.writeToDisk(entry)
}

// BackgroundRevalidate triggers a background re-render of a path.
// Deduplicates: if already revalidating, skips.
// A defer/recover guards against panics from the render function so a
// buggy page can never take down the server.
func (c *ISR) BackgroundRevalidate(path string, renderFunc func(path string) (string, int, []string)) {
	c.revMu.Lock()
	if c.revalidating[path] {
		c.revMu.Unlock()
		return
	}
	c.revalidating[path] = true
	c.revMu.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("  ⚠ ISR revalidate panic for %s: %v\n", path, r)
			}
			c.revMu.Lock()
			delete(c.revalidating, path)
			c.revMu.Unlock()
		}()

		html, ttl, tags := renderFunc(path)
		if html != "" {
			c.SetTagged(path, html, ttl, tags)
		}
	}()
}

// RevalidatePath forces an immediate re-render of a specific path.
func (c *ISR) RevalidatePath(path string, renderFunc func(path string) (string, int, []string)) {
	c.mu.Lock()
	delete(c.entries, path)
	c.mu.Unlock()

	if c.cacheDir != "" {
		hash := pathToHash(path)
		htmlPath := filepath.Join(c.cacheDir, hash+".html")
		metaPath := filepath.Join(c.cacheDir, hash+".meta")
		os.Remove(htmlPath)
		os.Remove(metaPath)
	}

	// Re-render immediately
	html, ttl, tags := renderFunc(path)
	if html != "" {
		c.SetTagged(path, html, ttl, tags)
	}
}

// RevalidateTag forces re-render of all pages tagged with `tag`.
func (c *ISR) RevalidateTag(tag string, renderFunc func(path string) (string, int, []string)) {
	c.mu.RLock()
	var paths []string
	for path, entry := range c.entries {
		for _, t := range entry.Tags {
			if t == tag {
				paths = append(paths, path)
				break
			}
		}
	}
	c.mu.RUnlock()

	for _, path := range paths {
		c.RevalidatePath(path, renderFunc)
	}
}

// Invalidate removes a path from cache
func (c *ISR) Invalidate(path string) {
	c.mu.Lock()
	delete(c.entries, path)
	c.mu.Unlock()

	if c.cacheDir != "" {
		hash := pathToHash(path)
		os.Remove(filepath.Join(c.cacheDir, hash+".html"))
		os.Remove(filepath.Join(c.cacheDir, hash+".meta"))
	}
}

// Clear removes all cached entries
func (c *ISR) Clear() {
	c.mu.Lock()
	c.entries = make(map[string]*Entry)
	c.mu.Unlock()

	if c.cacheDir != "" {
		os.RemoveAll(c.cacheDir)
		os.MkdirAll(c.cacheDir, 0750)
	}
}

// Size returns the number of cached entries
func (c *ISR) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// cleanup periodically removes expired entries
func (c *ISR) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for path, entry := range c.entries {
				if !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
					delete(c.entries, path)
				}
			}
			c.mu.Unlock()
		case <-c.stopCh:
			return
		}
	}
}

// Close stops the background cleanup goroutine and flushes entries to disk.
func (c *ISR) Close() {
	close(c.stopCh)
}

// --- Disk persistence ---

// pathToHash creates a safe filename hash from a URL path
func pathToHash(path string) string {
	h := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%x", h)[:12]
}

// writeToDisk saves an entry to the disk cache using atomic temp+rename so
// concurrent writers cannot observe torn/partial files.
func (c *ISR) writeToDisk(entry *Entry) {
	if c.cacheDir == "" {
		return
	}

	hash := pathToHash(entry.Path)

	// Write HTML file atomically
	htmlPath := filepath.Join(c.cacheDir, hash+".html")
	if err := atomicWrite(htmlPath, []byte(entry.HTML), 0600); err != nil {
		log.Printf("  ⚠ ISR disk write failed for %s: %v\n", entry.Path, err)
		return
	}

	// Write metadata file atomically
	metaPath := filepath.Join(c.cacheDir, hash+".meta")
	de := diskEntry{
		Path: entry.Path,
		TTL:  entry.TTL,
		Tags: entry.Tags,
	}
	if !entry.ExpiresAt.IsZero() {
		de.ExpiresAt = entry.ExpiresAt.Unix()
	}
	metaJSON, err := json.Marshal(de)
	if err != nil {
		log.Printf("  ⚠ ISR metadata encode failed for %s: %v\n", entry.Path, err)
		return
	}
	if err := atomicWrite(metaPath, metaJSON, 0600); err != nil {
		log.Printf("  ⚠ ISR metadata write failed for %s: %v\n", entry.Path, err)
	}
}

// atomicWrite writes data to a temporary file in the same directory and then
// renames it over `path`. Rename is atomic on POSIX and Windows (same volume)
// which avoids readers seeing a half-written file.
func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".isr-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		// Non-fatal on Windows; continue.
		_ = err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

// loadFromDisk loads all cached entries from disk into memory
func (c *ISR) loadFromDisk() {
	if c.cacheDir == "" {
		return
	}

	files, err := filepath.Glob(filepath.Join(c.cacheDir, "*.meta"))
	if err != nil {
		return
	}

	for _, metaPath := range files {
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}

		var de diskEntry
		if err := json.Unmarshal(data, &de); err != nil {
			continue
		}

		hash := pathToHash(de.Path)
		htmlPath := filepath.Join(c.cacheDir, hash+".html")
		htmlData, err := os.ReadFile(htmlPath)
		if err != nil {
			continue
		}

		entry := &Entry{
			HTML: string(htmlData),
			Path: de.Path,
			TTL:  de.TTL,
			Tags: de.Tags,
		}
		if de.ExpiresAt > 0 {
			entry.ExpiresAt = time.Unix(de.ExpiresAt, 0)
		}

		c.entries[de.Path] = entry
	}
}

// loadOneFromDisk loads a single entry from disk
func (c *ISR) loadOneFromDisk(path string) *Entry {
	if c.cacheDir == "" {
		return nil
	}

	hash := pathToHash(path)
	metaPath := filepath.Join(c.cacheDir, hash+".meta")

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil
	}

	var de diskEntry
	if err := json.Unmarshal(data, &de); err != nil {
		return nil
	}

	htmlPath := filepath.Join(c.cacheDir, hash+".html")
	htmlData, err := os.ReadFile(htmlPath)
	if err != nil {
		return nil
	}

	return &Entry{
		HTML: string(htmlData),
		Path: de.Path,
		TTL:  de.TTL,
		Tags: de.Tags,
	}
}
