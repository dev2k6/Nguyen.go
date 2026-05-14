package event

import (
	"log"
	"sync"
)

type Handler func(payload interface{})

type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

func NewBus() *Bus {
	return &Bus{
		handlers: make(map[string][]Handler),
	}
}

func (b *Bus) On(event string, handler Handler) {
	b.mu.Lock()
	b.handlers[event] = append(b.handlers[event], handler)
	b.mu.Unlock()
}

func (b *Bus) Emit(event string, payload interface{}) {
	b.mu.RLock()
	handlers := b.handlers[event]
	b.mu.RUnlock()

	for _, h := range handlers {
		h(payload)
	}
}

func (b *Bus) EmitAsync(event string, payload interface{}) {
	b.mu.RLock()
	handlers := b.handlers[event]
	b.mu.RUnlock()

	for _, h := range handlers {
		go func(fn Handler) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("  event: panic in handler for %q: %v", event, r)
				}
			}()
			fn(payload)
		}(h)
	}
}

func (b *Bus) Off(event string) {
	b.mu.Lock()
	delete(b.handlers, event)
	b.mu.Unlock()
}

func (b *Bus) HasListeners(event string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.handlers[event]) > 0
}

func (b *Bus) ListenerCount(event string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.handlers[event])
}
