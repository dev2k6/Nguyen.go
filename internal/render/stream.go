package render

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/dev2k6/Nguyen.go/internal/parser"
	"github.com/dev2k6/Nguyen.go/internal/router"
)

// ====================================================================
// Streaming SSR
// ====================================================================

// StreamShell sends the HTML shell (head + body up to async data) as the
// first chunk of a Transfer-Encoding: chunked response, then flushes.
// After data resolves, the caller should call StreamDataInject with the
// resolved values, followed by StreamClose to finish the body.
//
// Usage pattern:
//
//	render.StreamShell(c, result, version)
//	data := <-asyncDataChannel
//	render.StreamDataInject(c, data)
//	render.StreamClose(c, result, "</div></body></html>")
func StreamShell(c *fiber.Ctx, result *Result, version string) {
	c.Set("Content-Type", "text/html; charset=utf-8")
	c.Set("Transfer-Encoding", "chunked")
	c.Set("X-Content-Type-Options", "nosniff")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		// Write DOCTYPE + head + opening body/app div
		shell := buildStreamingShell(result, version)
		w.WriteString(shell)
		w.Flush()
	})
}

// StreamDataInject writes a `<script>` chunk that injects page data
// into `window.__nguyenPageData` so the client shell can hydrate
// without another round-trip.
func StreamDataInject(c *fiber.Ctx, data map[string]interface{}) {
	if c.Context().Response.IsBodyStream() {
		// Only meaningful in a streaming context
		c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
			injectChunk := buildDataInjectScript(data)
			w.WriteString(injectChunk)
			w.Flush()
		})
	}
}

// StreamClose writes the remaining body content and closes the stream.
func StreamClose(c *fiber.Ctx, result *Result, closingHTML string) {
	if c.Context().Response.IsBodyStream() {
		c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
			// Remaining body HTML + closing tags + hydration script
			w.WriteString(closingHTML)
			w.WriteString(buildHydrationJS())
			w.WriteString("\n</body>\n</html>")
			w.Flush()
		})
	}
}

// buildStreamingShell generates the initial HTML up to (and including)
// the opening container where async content will later be injected.
func buildStreamingShell(result *Result, version string) string {
	var sb strings.Builder
	// Rough estimate: ~300 bytes base + metadata
	sb.Grow(512 + len(version)*2)

	sb.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	sb.WriteString("    <meta charset=\"UTF-8\">\n")
	sb.WriteString("    <meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n")
	sb.WriteString("    <meta name=\"X-Powered-By\" content=\"Nguyen.go V")
	sb.WriteString(htmlEscape(version))
	sb.WriteString("\">\n")

	if title, ok := result.Meta["title"]; ok {
		sb.WriteString("    <title>")
		sb.WriteString(htmlEscape(title))
		sb.WriteString("</title>\n")
	}
	if desc, ok := result.Meta["description"]; ok {
		sb.WriteString("    <meta name=\"description\" content=\"")
		sb.WriteString(htmlEscape(desc))
		sb.WriteString("\">\n")
	}

	sb.WriteString("    <link rel=\"stylesheet\" href=\"/styles/output.css\">\n")
	sb.WriteString("    <script src=\"/bridge/nguyen_bridge.js\" defer></script>\n")

	// PWA manifest link (if configured)
	sb.WriteString("    <link rel=\"manifest\" href=\"/manifest.json\">\n")

	sb.WriteString("</head>\n<body data-nguyen-hydrate>\n")
	sb.WriteString("    <div id=\"app\">\n")

	return sb.String()
}

// buildDataInjectScript creates a `<script>` chunk that assigns the
// resolved async data into `window.__nguyenPageData`.
func buildDataInjectScript(data map[string]interface{}) string {
	var sb strings.Builder
	// Rough estimate: ~80 bytes prefix + payload
	sb.Grow(128)

	sb.WriteString("<script>(function(){window.__nguyenPageData=window.__nguyenPageData||{};")
	for k, v := range data {
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			jsonBytes = []byte(`""`)
		}
		sb.WriteString("window.__nguyenPageData[")
		sb.WriteString(fmt.Sprintf("%q", k))
		sb.WriteString("]=")
		sb.Write(jsonBytes)
		sb.WriteString(";")
	}
	sb.WriteString("})();</script>\n")
	return sb.String()
}

// RenderSSRStream performs SSR in streaming mode.
// It parses the .gox file, extracts sync-renderable HTML, sends the shell,
// waits for async data, injects it, then closes.
//
// This is the high-level convenience function used by the dev server.
func RenderSSRStream(c *fiber.Ctx, f *parser.File, version string, layouts ...router.LayoutInfo) error {
	result := RenderSSR(f, version, layouts...)

	// Split result.HTML into shell and closing parts.
	// For simplicity we wrap the whole RenderSSR output in a streaming envelope.
	shell := buildStreamingShell(result, version)
	body := result.HTML

	// Strip existing head/html wrappers if present (so we don't nest them)
	body = stripDocShell(body)

	c.Set("Content-Type", "text/html; charset=utf-8")
	c.Set("Transfer-Encoding", "chunked")
	c.Set("X-Content-Type-Options", "nosniff")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		// Phase 1: Shell
		w.WriteString(shell)
		w.Flush()

		// Phase 2: Body content (already rendered sync)
		w.WriteString(body)
		w.Flush()

		// Phase 3: Hydration script + close
		w.WriteString(buildHydrationJS())
		w.WriteString("\n</body>\n</html>")
		w.Flush()
	})

	return nil
}

// stripDocShell removes outer <!DOCTYPE>, <html>, <head>, and <body> tags
// from an HTML string so it can be re-wrapped in a streaming shell.
func stripDocShell(html string) string {
	html = strings.TrimSpace(html)
	// Remove DOCTYPE
	if idx := strings.Index(html, "<!DOCTYPE"); idx != -1 {
		end := strings.Index(html[idx:], ">")
		if end != -1 {
			html = html[:idx] + html[idx+end+1:]
		}
	}
	// Remove <html ...>
	html = stripTag(html, "html")
	// Remove <head>...</head>
	html = stripTag(html, "head")
	// Remove <body ...> and </body>
	html = stripTag(html, "body")
	return strings.TrimSpace(html)
}

// stripTag removes the outer occurrence of `<tag ...>` and `</tag>`.
func stripTag(s, tag string) string {
	openStart := strings.Index(strings.ToLower(s), "<"+tag)
	if openStart != -1 {
		openEnd := strings.Index(s[openStart:], ">")
		if openEnd != -1 {
			s = s[:openStart] + s[openStart+openEnd+1:]
		}
	}
	closeTag := "</" + tag + ">"
	closeIdx := strings.Index(strings.ToLower(s), closeTag)
	if closeIdx != -1 {
		s = s[:closeIdx] + s[closeIdx+len(closeTag):]
	}
	return s
}
