package server

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FileEvent represents a filesystem change event
type FileEvent int

const (
	FileChanged FileEvent = iota
	FileCreated
	FileDeleted
)

// WatchCallback is called when a watched file changes
type WatchCallback func(path string, event FileEvent)

// Watcher polls directories and notifies on file changes
type Watcher struct {
	dirs     []string
	callback WatchCallback
	interval time.Duration
	exts     map[string]bool
	modTimes map[string]time.Time
	mu       sync.Mutex
	stopCh   chan struct{}
}

// NewWatcher creates a file watcher that polls at the given interval
func NewWatcher(dirs []string, callback WatchCallback, interval time.Duration) *Watcher {
	exts := map[string]bool{
		".nguyen": true,
		".css":    true,
		".js":     true,
		".go":     true,
	}
	return &Watcher{
		dirs:     dirs,
		callback: callback,
		interval: interval,
		exts:     exts,
		modTimes: make(map[string]time.Time),
		stopCh:   make(chan struct{}),
	}
}

// Start begins polling for changes. Runs in a goroutine.
func (w *Watcher) Start() {
	// Initial scan — record all mod times
	w.scanAll()

	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.poll()
			case <-w.stopCh:
				return
			}
		}
	}()
}

// Stop stops the watcher
func (w *Watcher) Stop() {
	close(w.stopCh)
}

// scanAll records modification times for all watched files
func (w *Watcher) scanAll() {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, dir := range w.dirs {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // skip inaccessible files
			}
			if info != nil && info.IsDir() && !w.shouldTrack(path, info) {
				return filepath.SkipDir
			}
			if !w.shouldTrack(path, info) {
				return nil
			}
			w.modTimes[path] = info.ModTime()
			return nil
		})
	}
}

// poll checks all watched directories for changed files
func (w *Watcher) poll() {
	for _, dir := range w.dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info != nil && info.IsDir() && !w.shouldTrack(path, info) {
				return filepath.SkipDir
			}
			if !w.shouldTrack(path, info) {
				return nil
			}

			w.mu.Lock()
			prev, exists := w.modTimes[path]
			current := info.ModTime()

			if !exists {
				w.modTimes[path] = current
				w.mu.Unlock()
				w.callback(path, FileCreated)
				return nil
			}

			if current.After(prev) {
				w.modTimes[path] = current
				w.mu.Unlock()
				w.callback(path, FileChanged)
				return nil
			}

			w.mu.Unlock()
			return nil
		})
	}

	w.mu.Lock()
	for path := range w.modTimes {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			delete(w.modTimes, path)
			w.callback(path, FileDeleted)
		}
	}
	w.mu.Unlock()
}

func (w *Watcher) shouldTrack(path string, info os.FileInfo) bool {
	if info == nil {
		return false
	}

	name := info.Name()
	lowerName := strings.ToLower(name)

	if info.IsDir() {
		if lowerName == ".nguyen" || lowerName == ".git" || lowerName == "node_modules" || lowerName == ".idea" || lowerName == ".vscode" {
			return false
		}
		return true
	}

	// Skip editor temp / swap / OS metadata files — these trigger
	// spurious HMR reloads when the user saves in Vim/VS Code.
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "~") ||
		strings.HasPrefix(name, ".#") || strings.HasSuffix(name, "~") ||
		strings.HasSuffix(lowerName, ".swp") || strings.HasSuffix(lowerName, ".swo") ||
		strings.HasSuffix(lowerName, ".tmp") || strings.HasSuffix(lowerName, ".bak") {
		return false
	}
	if lowerName == ".ds_store" || lowerName == "thumbs.db" {
		return false
	}

	normalized := strings.ToLower(filepath.ToSlash(path))
	if strings.Contains(normalized, "/.nguyen/") || strings.Contains(normalized, "/.git/") || strings.Contains(normalized, "/node_modules/") {
		return false
	}

	ext := strings.ToLower(filepath.Ext(name))
	return w.exts[ext]
}
