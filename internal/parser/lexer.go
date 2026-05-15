package parser

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// parseCacheEntry holds a parsed file alongside its mtime for cache invalidation.
type parseCacheEntry struct {
	mtime int64
	file  *File
	err   error
}

var (
	parseCacheMu sync.RWMutex
	parseCache   = make(map[string]*parseCacheEntry)
)

// File represents a parsed .gox file
type File struct {
	Path         string     // Original file path
	GoCode       string     // Go code in frontmatter section
	HTMLTemplate string     // HTML template below frontmatter (scoped CSS already processed)
	RawTemplate  string     // Original HTML template before scoped CSS processing
	LineOffset   int        // Line offset for accurate error reporting
	ScopedCSS    *ScopedCSS // Scoped CSS data if <style scoped> was found
	IsLive       bool       // true when file uses Live Mode (.live.gox suffix or //+nguyen:live directive)
}

// Parse reads and parses a .gox file, separating frontmatter from template.
// Results are cached by file modification time to avoid repeated disk reads and re-parses.
func Parse(path string) (*File, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", path, err)
	}
	mtime := stat.ModTime().Unix()

	parseCacheMu.RLock()
	entry, ok := parseCache[path]
	parseCacheMu.RUnlock()
	if ok && entry.mtime == mtime {
		return entry.file, entry.err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		e := &parseCacheEntry{mtime: mtime, err: fmt.Errorf("failed to read file %s: %w", path, err)}
		parseCacheMu.Lock()
		parseCache[path] = e
		parseCacheMu.Unlock()
		return nil, e.err
	}

	f, err := parseString(string(content), path)
	e := &parseCacheEntry{mtime: mtime, file: f, err: err}
	parseCacheMu.Lock()
	parseCache[path] = e
	parseCacheMu.Unlock()
	return f, err
}

// parseString parses the string content into a File
func parseString(content, path string) (*File, error) {
	lines := strings.Split(content, "\n")

	// Find the first "---" line
	firstIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			firstIdx = i
			break
		}
	}

	// No frontmatter - entire file is template
	if firstIdx == -1 {
		processed, scoped := ProcessScopedCSS(path, content)
		return &File{
			Path:         path,
			GoCode:       "",
			HTMLTemplate: processed,
			RawTemplate:  content,
			LineOffset:   1,
			ScopedCSS:    scoped[path],
			IsLive:       isLivePath(path),
		}, nil
	}

	// Find the second "---" line
	secondIdx := -1
	for i := firstIdx + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			secondIdx = i
			break
		}
	}

	if secondIdx == -1 {
		return nil, fmt.Errorf("syntax error in %s: missing closing --- for frontmatter (line %d)", path, firstIdx+1)
	}

	// Extract Go code and template
	goCode := strings.Join(lines[firstIdx+1:secondIdx], "\n")
	rawTemplate := strings.Join(lines[secondIdx+1:], "\n")
	htmlTemplate := strings.TrimSpace(rawTemplate)

	// Process scoped CSS
	processed, scoped := ProcessScopedCSS(path, htmlTemplate)

	goCodeTrimmed := strings.TrimSpace(goCode)

	return &File{
		Path:         path,
		GoCode:       goCodeTrimmed,
		HTMLTemplate: processed,
		RawTemplate:  rawTemplate,
		LineOffset:   secondIdx + 2,
		ScopedCSS:    scoped[path],
		IsLive:       isLivePath(path) || hasLiveDirective(goCodeTrimmed),
	}, nil
}

// isLivePath reports whether a file path opts into Live Mode by suffix.
// Pages named "<name>.live.gox" use Live Mode.
func isLivePath(path string) bool {
	return strings.HasSuffix(path, ".live.gox")
}

// hasLiveDirective reports whether a frontmatter Go block opts into
// Live Mode via a //+nguyen:live comment. The directive may be on its
// own line or trailing whitespace; matching is whitespace-tolerant.
func hasLiveDirective(goCode string) bool {
	for _, line := range strings.Split(goCode, "\n") {
		line = strings.TrimSpace(line)
		if line == "//+nguyen:live" || line == "// +nguyen:live" ||
			strings.HasPrefix(line, "//+nguyen:live ") ||
			strings.HasPrefix(line, "// +nguyen:live ") {
			return true
		}
	}
	return false
}

// Name returns the component name derived from the file path
// e.g.: pages/index.gox → "index", pages/product.gox → "product"
func (f *File) Name() string {
	fileName := f.Path

	// Get file name without directory
	idx := strings.LastIndex(fileName, "/")
	if idx == -1 {
		idx = strings.LastIndex(fileName, "\\")
	}
	if idx != -1 {
		fileName = fileName[idx+1:]
	}

	// Remove .gox extension
	fileName = strings.TrimSuffix(fileName, ".gox")
	return fileName
}

// PackageName creates a valid Go package name from the file path
func (f *File) PackageName() string {
	name := f.Name()
	// Replace special characters
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, "[", "")
	name = strings.ReplaceAll(name, "]", "")
	return "page_" + name
}
