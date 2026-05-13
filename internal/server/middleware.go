package server

import (
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Middleware represents a named middleware function.
type Middleware struct {
	Name    string
	Handler fiber.Handler
}

// MiddlewareRegistry holds registered middleware by name.
// Route guards declared in .gox frontmatter (guard = "auth") are resolved
// against this registry at request time.
type MiddlewareRegistry struct {
	mu          sync.RWMutex
	middlewares map[string]fiber.Handler
}

// NewMiddlewareRegistry creates an empty registry.
func NewMiddlewareRegistry() *MiddlewareRegistry {
	return &MiddlewareRegistry{
		middlewares: make(map[string]fiber.Handler),
	}
}

// Register adds a named middleware to the registry.
func (r *MiddlewareRegistry) Register(name string, handler fiber.Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middlewares[name] = handler
}

// Get retrieves a middleware by name. Returns nil if not found.
func (r *MiddlewareRegistry) Get(name string) fiber.Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.middlewares[name]
}

// Resolve returns an ordered slice of handlers for a comma-separated guard string.
// Example: "auth,admin" resolves to [authHandler, adminHandler].
func (r *MiddlewareRegistry) Resolve(guards string) []fiber.Handler {
	if guards == "" {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := strings.Split(guards, ",")
	var handlers []fiber.Handler
	for _, name := range names {
		name = strings.TrimSpace(name)
		if h, ok := r.middlewares[name]; ok {
			handlers = append(handlers, h)
		}
	}
	return handlers
}

// CORS returns a middleware that sets CORS headers.
// Configurable origins, methods, and headers.
func CORS(allowOrigins string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", allowOrigins)
		c.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		c.Set("Access-Control-Max-Age", "86400")

		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(204)
		}
		return c.Next()
	}
}

// RequestID adds a unique request ID header to each response.
func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Get("X-Request-ID")
		if id == "" {
			id = generateRequestID()
		}
		c.Set("X-Request-ID", id)
		c.Locals("requestID", id)
		return c.Next()
	}
}

func generateRequestID() string {
	// Fast pseudo-unique ID using timestamp + goroutine-safe counter
	var counter uint64
	counterMu.Lock()
	counter = requestCounter
	requestCounter++
	counterMu.Unlock()

	const chars = "0123456789abcdef"
	ts := uint64(time.Now().UnixNano())
	var buf [16]byte
	for i := 7; i >= 0; i-- {
		buf[i] = chars[ts&0xf]
		ts >>= 4
	}
	for i := 15; i >= 8; i-- {
		buf[i] = chars[counter&0xf]
		counter >>= 4
	}
	return string(buf[:])
}

var (
	counterMu      sync.Mutex
	requestCounter uint64
)
