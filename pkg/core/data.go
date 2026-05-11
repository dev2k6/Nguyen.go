//go:build js && wasm

package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"syscall/js"
)

// ====================================================================
// Page Data (Server-Side Props injection)
// ====================================================================

var (
	pageData        map[string]interface{}
	pageDataFetched = false
)

// SetPageData injects server-side props into the WASM runtime.
// Called by the bridge.js bootstrap before app hydration.
func SetPageData(data map[string]interface{}) {
	pageData = data
	pageDataFetched = true
}

// UsePageData retrieves a value from server-injected page data.
// If the key does not exist, returns nil.
//
// Usage:
//
//	props := core.UsePageData()
//	title := props["title"]
func UsePageData() map[string]interface{} {
	if !pageDataFetched {
		// Try to read from window.__nguyenPageData injected during SSR
		ngData := js.Global().Get("__nguyenPageData")
		if !ngData.IsUndefined() && !ngData.IsNull() {
			jsonStr := ngData.Call("toString").String()
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
				pageData = parsed
			}
		}
		pageDataFetched = true
	}
	if pageData == nil {
		return make(map[string]interface{})
	}
	return pageData
}

// GetPageData retrieves a specific key from page data.
func GetPageData(key string) interface{} {
	d := UsePageData()
	if v, ok := d[key]; ok {
		return v
	}
	return nil
}

// ====================================================================
// Query Hook (useQuery equivalent)
// ====================================================================

type QueryResult struct {
	Data      interface{}
	IsLoading bool
	Error     error
}

type queryCacheEntry struct {
	data      interface{}
	timestamp int64
}

var queryCache = make(map[string]*queryCacheEntry)

// UseQuery fetches data via an async fetcher function and caches it by key.
// Equivalent to React Query / SWR's useQuery.
//
// Usage:
//
//	result := core.UseQuery("users", func() (interface{}, error) {
//	    return fetchUsers()
//	})
//	if result.IsLoading { ... }
//	if result.Error != nil { ... }
func UseQuery(key string, fetcher func() (interface{}, error)) *QueryResult {
	data, setData := UseState(nil)
	loading, setLoading := UseState(true)
	err, setErr := UseState(nil)

	UseEffect(func() interface{} {
		go func() {
			// Check cache first
			if entry, ok := queryCache[key]; ok {
				setData(entry.data)
				setLoading(false)
				return
			}
			result, fetchErr := fetcher()
			if fetchErr != nil {
				setErr(fetchErr)
			} else {
				queryCache[key] = &queryCacheEntry{data: result}
				setData(result)
			}
			setLoading(false)
		}()
		return nil
	}, []interface{}{key})

	qErr, _ := err.(error)
	return &QueryResult{
		Data:      data,
		IsLoading: loading.(bool),
		Error:     qErr,
	}
}

// InvalidateQuery clears the cache for a given key, forcing refetch on next UseQuery.
func InvalidateQuery(key string) {
	delete(queryCache, key)
}

// ====================================================================
// Mutation Hook (useMutation equivalent)
// ====================================================================

type MutationResult struct {
	Data      interface{}
	IsLoading bool
	Error     error
}

// UseMutation returns a mutator function and its result state.
// Call mutator(input) to execute the mutation.
//
// Usage:
//
//	result, mutate := core.UseMutation(func(input interface{}) (interface{}, error) {
//	    return createUser(input)
//	})
//	mutate(userInput)
func UseMutation(mutator func(interface{}) (interface{}, error)) (*MutationResult, func(interface{})) {
	data, setData := UseState(nil)
	loading, setLoading := UseState(false)
	err, setErr := UseState(nil)

	mutate := func(input interface{}) {
		setLoading(true)
		setErr(nil)
		go func() {
			result, fetchErr := mutator(input)
			if fetchErr != nil {
				setErr(fetchErr)
			} else {
				setData(result)
			}
			setLoading(false)
		}()
	}

	qErr, _ := err.(error)
	return &MutationResult{
		Data:      data,
		IsLoading: loading.(bool),
		Error:     qErr,
	}, mutate
}

// ====================================================================
// LocalStorage Hook
// ====================================================================

// UseLocalStorage synchronizes a state value with the browser's localStorage.
// Persists across page reloads and syncs across browser tabs.
//
// Usage:
//
//	count, setCount := core.UseLocalStorage("myCounter", 0)
func UseLocalStorage(key string, initialValue interface{}) (interface{}, func(interface{})) {
	// Read initial value from localStorage if present
	stored := js.Global().Get("localStorage").Call("getItem", key)
	var startValue interface{} = initialValue
	if !stored.IsNull() && !stored.IsUndefined() {
		startValue = stored.String()
	}

	value, setValue := UseState(startValue)

	// Persist changes to localStorage
	UseEffect(func() interface{} {
		if v := value; v != nil {
			js.Global().Get("localStorage").Call("setItem", key, fmtString(v))
		}
		return nil
	}, []interface{}{value, key})

	// Listen for storage events from other tabs
	UseEffect(func() interface{} {
		handler := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			event := args[0]
			if event.Get("key").String() == key {
				newVal := event.Get("newValue").String()
				if newVal != "" {
					setValue(newVal)
				}
			}
			return nil
		})
		js.Global().Get("window").Call("addEventListener", "storage", handler)
		return func() {
			js.Global().Get("window").Call("removeEventListener", "storage", handler)
		}
	}, []interface{}{key})

	return value, setValue
}

// UseSessionStorage is identical to UseLocalStorage but uses sessionStorage.
func UseSessionStorage(key string, initialValue interface{}) (interface{}, func(interface{})) {
	stored := js.Global().Get("sessionStorage").Call("getItem", key)
	var startValue interface{} = initialValue
	if !stored.IsNull() && !stored.IsUndefined() {
		startValue = stored.String()
	}

	value, setValue := UseState(startValue)

	UseEffect(func() interface{} {
		if v := value; v != nil {
			js.Global().Get("sessionStorage").Call("setItem", key, fmtString(v))
		}
		return nil
	}, []interface{}{value, key})

	return value, setValue
}

// fmtString converts an interface{} to a string for storage.
func fmtString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// FetchJSON performs a GET request using the browser's fetch API and returns parsed JSON.
func FetchJSON(url string) (map[string]interface{}, error) {
	done := make(chan struct{})
	var result map[string]interface{}
	var fetchErr error

	then := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		resp := args[0]
		if !resp.Get("ok").Bool() {
			fetchErr = fmt.Errorf("fetch failed: %d %s", resp.Get("status").Int(), resp.Get("statusText").String())
			close(done)
			return nil
		}
		resp.Call("text").Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			body := args[0].String()
			if err := json.Unmarshal([]byte(body), &result); err != nil {
				fetchErr = err
			}
			close(done)
			return nil
		}))
		return nil
	})

	catch := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		fetchErr = errors.New(args[0].Get("message").String())
		close(done)
		return nil
	})

	js.Global().Call("fetch", url).Call("then", then).Call("catch", catch)
	<-done

	return result, fetchErr
}
