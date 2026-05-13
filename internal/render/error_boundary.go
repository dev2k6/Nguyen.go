package render

import (
	"fmt"
	"log"
	"strings"
)

// ErrorBoundary wraps page rendering with graceful error recovery.
// If a page render panics or returns an error, the boundary catches it
// and renders a fallback instead of crashing the server.
type ErrorBoundary struct {
	FallbackHTML string
	OnError      func(path string, err error)
}

// DefaultErrorBoundary returns a boundary with a minimal fallback page.
func DefaultErrorBoundary() *ErrorBoundary {
	return &ErrorBoundary{
		FallbackHTML: defaultErrorHTML,
		OnError: func(path string, err error) {
			log.Printf("  ⚠ Render error on %s: %v\n", path, err)
		},
	}
}

// Render executes renderFn within the error boundary. If renderFn panics
// or returns an error, the fallback HTML is returned instead.
func (eb *ErrorBoundary) Render(path string, renderFn func() (string, error)) string {
	var html string
	var err error

	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic: %v", r)
			}
		}()
		html, err = renderFn()
	}()

	if err != nil {
		if eb.OnError != nil {
			eb.OnError(path, err)
		}
		return eb.renderFallback(path, err)
	}

	return html
}

func (eb *ErrorBoundary) renderFallback(path string, err error) string {
	fallback := eb.FallbackHTML
	fallback = strings.ReplaceAll(fallback, "{{path}}", htmlEscape(path))
	fallback = strings.ReplaceAll(fallback, "{{error}}", htmlEscape(err.Error()))
	return fallback
}

var defaultErrorHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Error — Nguyen.go</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: system-ui, -apple-system, sans-serif; min-height: 100vh;
               display: flex; align-items: center; justify-content: center;
               background: #fafafa; color: #333; }
        .error-box { text-align: center; padding: 3rem; max-width: 480px; }
        .error-icon { font-size: 3rem; margin-bottom: 1rem; }
        .error-title { font-size: 1.25rem; font-weight: 600; margin-bottom: 0.5rem; }
        .error-path { font-size: 0.875rem; color: #666; margin-bottom: 1rem;
                      font-family: monospace; background: #f0f0f0; padding: 0.25rem 0.5rem;
                      border-radius: 4px; display: inline-block; }
        .error-msg { font-size: 0.875rem; color: #999; }
        .error-retry { margin-top: 1.5rem; }
        .error-retry a { color: #6366f1; text-decoration: none; font-weight: 500; }
        .error-retry a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <div class="error-box">
        <div class="error-icon">⚠</div>
        <div class="error-title">Something went wrong</div>
        <div class="error-path">{{path}}</div>
        <div class="error-msg">{{error}}</div>
        <div class="error-retry"><a href="javascript:location.reload()">Try again</a></div>
    </div>
</body>
</html>`
