package render

import (
	"context"
	"sync"
	"time"
)

// LoaderFunc is a function that fetches data for a page during SSR.
// It receives a context with timeout and request metadata, and returns
// data that will be injected into the template rendering context.
type LoaderFunc func(ctx context.Context, params LoaderParams) (interface{}, error)

// LoaderParams provides request context to data loaders.
type LoaderParams struct {
	Path    string
	Params  map[string]string
	Query   map[string]string
	Headers map[string]string
}

// LoaderResult holds the output of a data loader execution.
type LoaderResult struct {
	Data  interface{}
	Error error
	Key   string
}

// LoaderRegistry manages named data loaders for SSR pages.
type LoaderRegistry struct {
	mu      sync.RWMutex
	loaders map[string]LoaderFunc
	timeout time.Duration
}

// NewLoaderRegistry creates a registry with a default timeout.
func NewLoaderRegistry(timeout time.Duration) *LoaderRegistry {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &LoaderRegistry{
		loaders: make(map[string]LoaderFunc),
		timeout: timeout,
	}
}

// Register adds a named data loader.
func (r *LoaderRegistry) Register(name string, fn LoaderFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loaders[name] = fn
}

// Execute runs multiple loaders concurrently and returns all results.
// Leverages Go goroutines for parallel data fetching — each loader
// runs in its own goroutine with a shared context deadline.
func (r *LoaderRegistry) Execute(names []string, params LoaderParams) []LoaderResult {
	if len(names) == 0 {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	results := make([]LoaderResult, len(names))
	var wg sync.WaitGroup

	for i, name := range names {
		fn, ok := r.loaders[name]
		if !ok {
			results[i] = LoaderResult{Key: name, Error: ErrLoaderNotFound}
			continue
		}

		wg.Add(1)
		go func(idx int, key string, loader LoaderFunc) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					results[idx] = LoaderResult{Key: key, Error: ErrLoaderPanic}
				}
			}()

			data, err := loader(ctx, params)
			results[idx] = LoaderResult{Key: key, Data: data, Error: err}
		}(i, name, fn)
	}

	wg.Wait()
	return results
}

// ExecuteSingle runs a single loader by name.
func (r *LoaderRegistry) ExecuteSingle(name string, params LoaderParams) *LoaderResult {
	results := r.Execute([]string{name}, params)
	if len(results) == 0 {
		return &LoaderResult{Key: name, Error: ErrLoaderNotFound}
	}
	return &results[0]
}

type loaderError string

func (e loaderError) Error() string { return string(e) }

const (
	ErrLoaderNotFound loaderError = "loader not found"
	ErrLoaderPanic    loaderError = "loader panicked"
	ErrLoaderTimeout  loaderError = "loader timed out"
)
