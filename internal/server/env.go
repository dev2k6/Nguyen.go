package server

import (
	"os"
	"strings"
	"sync"
)

// Env provides safe access to environment variables for SSR rendering.
// Only variables prefixed with NGUYEN_ are exposed to prevent leaking secrets.
type Env struct {
	mu     sync.RWMutex
	cache  map[string]string
	prefix string
}

var globalEnv = &Env{
	cache:  make(map[string]string),
	prefix: "NGUYEN_",
}

// GetEnv returns the global environment accessor.
func GetEnv() *Env {
	return globalEnv
}

// Get retrieves an environment variable by key.
// Only NGUYEN_ prefixed vars are accessible. Returns empty string if not found.
func (e *Env) Get(key string) string {
	e.mu.RLock()
	if val, ok := e.cache[key]; ok {
		e.mu.RUnlock()
		return val
	}
	e.mu.RUnlock()

	fullKey := key
	if !strings.HasPrefix(key, e.prefix) {
		fullKey = e.prefix + key
	}

	val := os.Getenv(fullKey)
	if val != "" {
		e.mu.Lock()
		e.cache[key] = val
		e.mu.Unlock()
	}
	return val
}

// All returns all NGUYEN_ prefixed environment variables as a map.
// Safe to expose to client-side rendering context.
func (e *Env) All() map[string]string {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.HasPrefix(parts[0], e.prefix) {
			key := strings.TrimPrefix(parts[0], e.prefix)
			e.cache[key] = parts[1]
		}
	}

	result := make(map[string]string, len(e.cache))
	for k, v := range e.cache {
		result[k] = v
	}
	return result
}

// IsProduction returns true if NGUYEN_ENV is "production".
func (e *Env) IsProduction() bool {
	return e.Get("ENV") == "production"
}

// IsDevelopment returns true if NGUYEN_ENV is empty or "development".
func (e *Env) IsDevelopment() bool {
	env := e.Get("ENV")
	return env == "" || env == "development"
}
