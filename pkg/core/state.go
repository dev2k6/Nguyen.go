//go:build js && wasm

package core

// StateManager is the global state management for the application
// Note: WASM is single-threaded, so sync.Mutex is not needed.
type StateManager struct {
	data       map[string]interface{}
	components map[string]func() // Registered components for re-rendering
}

// globalState is the singleton instance
var globalState = &StateManager{
	data:       make(map[string]interface{}),
	components: make(map[string]func()),
}

// UseGlobalState initializes or reads a global state value
// Returns (currentValue, setterFunction)
// Usage: count, setCount := core.UseGlobalState("count", 0)
func UseGlobalState(key string, initialValue interface{}) (interface{}, func(interface{})) {
	if val, ok := globalState.data[key]; ok {
		// State exists, return current value with setter
		return val, func(newVal interface{}) {
			globalState.data[key] = newVal
			triggerRerender(key)
		}
	}

	// Create new state
	globalState.data[key] = initialValue
	return initialValue, func(newVal interface{}) {
		globalState.data[key] = newVal
		triggerRerender(key)
	}
}

// GetGlobalState reads the current global state value (used in templates)
func GetGlobalState(key string) interface{} {
	return globalState.data[key]
}

// SetGlobalState updates a global state value
func SetGlobalState(key string, newValue interface{}) {
	globalState.data[key] = newValue
	triggerRerender(key)
}

// RegisterComponent registers a render function for a component
func RegisterComponent(key string, renderFn func()) {
	globalState.components[key] = renderFn
}

// triggerRerender calls the re-render function of a component when state changes
func triggerRerender(key string) {
	if render, ok := globalState.components[key]; ok {
		render()
	}
}

// Effect registers a callback that runs after each render
func Effect(key string, effectFn func()) {
	RegisterComponent(key, effectFn)
	// Run immediately on first mount
	effectFn()
}
