//go:build js && wasm

package core

import (
	"syscall/js"
)

var (
	loadedChunks  = make(map[string]bool)
	chunkPromises = make(map[string]js.Value)
	sharedModule  js.Value // shared WASM instance exports
)

// LoadShared fetches and instantiates the shared WASM module (core/DOM/Router).
// Called once at bootstrap. All page chunks import from this module.
// WASM is single-threaded, so no mutex needed.
func LoadShared() js.Value {
	if !sharedModule.IsNull() && !sharedModule.IsUndefined() {
		return sharedModule
	}

	// Fetch and instantiate shared.wasm
	resp := js.Global().Call("fetch", "/chunks/shared.wasm")
	promise := resp.Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		response := args[0]
		return response.Call("arrayBuffer")
	})).Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		buffer := args[0]
		return js.Global().Get("WebAssembly").Call("instantiate", buffer, goImportObject())
	})).Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		result := args[0]
		sharedModule = result.Get("instance").Get("exports")
		loadedChunks["shared"] = true
		return sharedModule
	}))

	return promise
}

// LoadChunk fetches and instantiates a page-specific .wasm file.
// The chunk is loaded via fetch → WebAssembly.instantiate with shared imports.
// Returns a JS Promise that resolves with the page's exports.
func LoadChunk(routePath string) js.Value {
	if loadedChunks[routePath] {
		return chunkPromises[routePath]
	}

	// Load shared first if not loaded
	if sharedModule.IsNull() || sharedModule.IsUndefined() {
		LoadShared()
	}

	// Determine chunk filename from route path
	chunkName := routeToChunkName(routePath)

	promise := js.Global().Call("fetch", "/chunks/"+chunkName).Call("then",
		js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			response := args[0]
			return response.Call("arrayBuffer")
		}),
	).Call("then",
		js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			buffer := args[0]
			return js.Global().Get("WebAssembly").Call("instantiate", buffer, goImportObject())
		}),
	).Call("then",
		js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			result := args[0]
			loadedChunks[routePath] = true
			return result.Get("instance").Get("exports")
		}),
	)

	chunkPromises[routePath] = promise
	return promise
}

// PrefetchChunk fetches a chunk in the background without instantiating it.
// Used for hover prefetch: data arrives but is not instantiated until navigation.
func PrefetchChunk(routePath string) {
	if loadedChunks[routePath] {
		return
	}
	chunkName := routeToChunkName(routePath)
	go func() {
		js.Global().Call("fetch", "/chunks/"+chunkName)
	}()
}

// IsChunkLoaded returns whether a route chunk has been loaded
func IsChunkLoaded(routePath string) bool {
	return loadedChunks[routePath]
}

// routeToChunkName converts a route path to a chunk filename
func routeToChunkName(path string) string {
	if path == "/" {
		return "index.wasm"
	}
	// Strip leading /, replace / with _, remove trailing /
	name := ""
	for i, c := range path {
		if i == 0 && c == '/' {
			continue
		}
		if c == '/' {
			name += "_"
		} else {
			name += string(c)
		}
	}
	if name == "" {
		return "index.wasm"
	}
	return name + ".wasm"
}

// goImportObject returns the Go WASM import object needed by TinyGo-compiled modules
func goImportObject() js.Value {
	goObj := js.Global().Get("go")
	if goObj.IsNull() || goObj.IsUndefined() {
		// Return minimal import object
		importObj := js.Global().Get("Object").New()
		env := js.Global().Get("Object").New()
		env.Set("abort", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			println("WASM abort:", args[0].Int())
			return nil
		}))
		importObj.Set("env", env)
		return importObj
	}
	return goObj.Get("importObject")
}
