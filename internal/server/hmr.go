package server

import (
	"bufio"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// HMRClient represents a connected SSE client
type HMRClient struct {
	id     string
	sendCh chan string
}

// HMRHub manages SSE clients for Hot Module Replacement
type HMRHub struct {
	clients map[string]*HMRClient
	mu      sync.RWMutex
}

// NewHMRHub creates a new HMR hub
func NewHMRHub() *HMRHub {
	return &HMRHub{
		clients: make(map[string]*HMRClient),
	}
}

// Register adds a new SSE client
func (h *HMRHub) Register(id string, ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[id] = &HMRClient{id: id, sendCh: ch}
}

// Unregister removes an SSE client
func (h *HMRHub) Unregister(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, id)
}

// Broadcast sends a message to all connected SSE clients
func (h *HMRHub) Broadcast(msg string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, client := range h.clients {
		select {
		case client.sendCh <- msg:
		default:
			// client is slow, skip this message
		}
	}
}

// ClientCount returns the number of connected clients
func (h *HMRHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// SSEHandler returns a Fiber handler for SSE connections.
// Clients connect via EventSource('/_nguyen/hmr') and receive
// events: 'css-reload', 'reload', or 'ping'.
func (h *HMRHub) SSEHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("X-Accel-Buffering", "no") // disable nginx buffering

		clientID := fmt.Sprintf("%s-%d", c.IP(), time.Now().UnixNano())
		ch := make(chan string, 10)
		h.Register(clientID, ch)
		defer h.Unregister(clientID)

		// Send initial connection event
		fmt.Fprintf(c, "event: connected\ndata: %d\n\n", h.ClientCount())
		c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case msg, ok := <-ch:
					if !ok {
						w.Flush()
						return
					}
					fmt.Fprintf(w, "event: %s\ndata: %s\n\n", msg, time.Now().Format(time.RFC3339))
					w.Flush()
				case <-ticker.C:
					fmt.Fprintf(w, "event: ping\ndata: pong\n\n")
					w.Flush()
				}
			}
		})

		return nil
	}
}

// SetupHMR creates the file watcher and SSE endpoint.
func SetupHMR(app *fiber.App, watchDirs []string) (*HMRHub, *Watcher) {
	hub := NewHMRHub()

	app.Get("/_nguyen/hmr", hub.SSEHandler())

	watcher := NewWatcher(watchDirs, func(path string, event FileEvent) {
		var action string
		ext := strings.ToLower(filepath.Ext(path))

		if ext == ".css" {
			action = "css-reload"
		} else {
			switch event {
			case FileChanged, FileCreated, FileDeleted:
				action = "reload"
			default:
				action = "reload"
			}
		}

		log.Printf("  HMR: %s %s -> broadcasting %s", action, path, action)
		hub.Broadcast(action)
	}, 1*time.Second)

	watcher.Start()

	return hub, watcher
}
