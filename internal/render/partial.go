package render

import (
	"encoding/binary"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dev2k6/Nguyen.go/internal/parser"
	"github.com/dev2k6/Nguyen.go/internal/router"
)

// PartialMeta holds metadata for a partial navigation response.
type PartialMeta struct {
	Title  string            `json:"title,omitempty"`
	Desc   string            `json:"desc,omitempty"`
	State  map[string]string `json:"state,omitempty"`
	Outlet int               `json:"outlet"`
	CSS    string            `json:"css,omitempty"`
}

// PartialResult holds the output of a partial page render for SPA navigation.
type PartialResult struct {
	HTML   string
	Meta   PartialMeta
	Layout string
}

// LayoutChain returns the ordered layout identifiers that apply to a page.
// Each entry is the relative directory path of the layout file.
func LayoutChain(pagePath string, layouts []router.LayoutInfo) []string {
	pageDir := filepath.ToSlash(filepath.Dir(pagePath))
	var chain []string
	for _, l := range layouts {
		lDir := filepath.ToSlash(l.DirPath)
		if strings.HasPrefix(pageDir, lDir) || lDir == "." {
			chain = append(chain, filepath.ToSlash(l.FilePath))
		}
	}
	return chain
}

// SharedLayoutDepth compares two layout chains and returns the depth at which
// they diverge. Content should be swapped at this outlet depth.
func SharedLayoutDepth(fromChain, toChain []string) int {
	depth := 0
	for i := 0; i < len(fromChain) && i < len(toChain); i++ {
		if fromChain[i] != toChain[i] {
			break
		}
		depth = i + 1
	}
	return depth
}

// RenderPartial renders only the page content that differs from the current
// layout context. It composes layouts only below the shared boundary.
func RenderPartial(f *parser.File, version string, layouts []router.LayoutInfo, fromPath string, allRoutes []router.Route) *PartialResult {
	result := &PartialResult{
		Meta: PartialMeta{
			State: make(map[string]string),
		},
	}

	// Extract metadata
	meta := make(map[string]string)
	extractMetadata(f.GoCode, meta)
	result.Meta.Title = meta["title"]
	result.Meta.Desc = meta["description"]

	// Extract state
	extractState(f.GoCode, result.Meta.State)

	// Build page HTML with variable interpolation
	html := f.HTMLTemplate
	state := result.Meta.State
	html = varRx.ReplaceAllStringFunc(html, func(match string) string {
		name := match[1 : len(match)-1]
		if val, ok := state[name]; ok {
			return `<span data-nguyen-text="` + name + `">` + htmlEscape(val) + `</span>`
		}
		return ""
	})

	// Transform <nguyen-link> → <a>
	html = nguyenLinkRx.ReplaceAllString(html, `<a$1>`)
	html = nguyenLinkEnd.ReplaceAllString(html, `</a>`)

	// Determine layout chains
	toChain := LayoutChain(f.Path, layouts)

	var fromChain []string
	if fromPath != "" {
		fromRoute, _, found := router.MatchRoute(allRoutes, fromPath)
		if found {
			fromChain = LayoutChain(fromRoute.FilePath, layouts)
		}
	}

	// Find shared layout depth
	sharedDepth := SharedLayoutDepth(fromChain, toChain)
	result.Meta.Outlet = sharedDepth

	// Compose only the layouts below the shared boundary
	if len(toChain) > sharedDepth {
		belowLayouts := toChain[sharedDepth:]
		// Apply layouts from deepest to shallowest (within the non-shared portion)
		for i := len(belowLayouts) - 1; i >= 0; i-- {
			layoutFile, err := parser.Parse(belowLayouts[i])
			if err != nil {
				continue
			}
			layoutHTML := layoutFile.HTMLTemplate
			// Mark outlet
			outletDepth := sharedDepth + i
			replacement := `<div data-nguyen-outlet="` + Itoa(outletDepth+1) + `">` + html + `</div>`
			html = slotRx.ReplaceAllString(layoutHTML, replacement)
		}
	}

	// Inject event markers
	html = eventAttrRx.ReplaceAllStringFunc(html, func(match string) string {
		m := eventAttrRx.FindStringSubmatch(match)
		if len(m) < 3 {
			return match
		}
		handlerName := strings.TrimSuffix(m[2], "()")
		return ` data-nguyen-` + m[1] + `="` + handlerName + `"`
	})

	result.HTML = html
	return result
}

// EncodePartialFrame encodes a partial result into the binary frame format:
// [4 bytes: metadata length (uint32 BE)] [N bytes: JSON metadata] [remaining: HTML]
func EncodePartialFrame(pr *PartialResult) ([]byte, error) {
	metaJSON, err := json.Marshal(pr.Meta)
	if err != nil {
		return nil, err
	}

	metaLen := len(metaJSON)
	frame := make([]byte, 4+metaLen+len(pr.HTML))
	binary.BigEndian.PutUint32(frame[0:4], uint32(metaLen))
	copy(frame[4:4+metaLen], metaJSON)
	copy(frame[4+metaLen:], pr.HTML)
	return frame, nil
}

// Itoa converts an int to string.
func Itoa(n int) string {
	return strconv.Itoa(n)
}
