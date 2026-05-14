package ws

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

type Config struct {
	Enabled       bool   `yaml:"enabled"`
	Path          string `yaml:"path"`
	MaxMessageSize int64 `yaml:"max_message_size"`
	PingInterval  int    `yaml:"ping_interval"`
}

type Message struct {
	Type    string          `json:"type"`
	Room    string          `json:"room,omitempty"`
	To      string          `json:"to,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type Client struct {
	ID      string
	Conn    *websocket.Conn
	Hub     *Hub
	Outbox  chan []byte
	rooms   map[string]bool
	mu      sync.RWMutex
}

type Room struct {
	ID      string
	clients map[string]*Client
	mu      sync.RWMutex
}

type Hub struct {
	config     Config
	clients    map[string]*Client
	rooms      map[string]*Room
	register   chan *Client
	unregister chan *Client
	broadcast  chan *broadcastMsg
	mu         sync.RWMutex
	onConnect    func(client *Client)
	onDisconnect func(client *Client)
	onMessage    func(client *Client, msg *Message)
}

type broadcastMsg struct {
	room    string
	data    []byte
	exclude string
}

func NewHub(cfg Config) *Hub {
	if cfg.Path == "" {
		cfg.Path = "/ws"
	}
	if cfg.MaxMessageSize == 0 {
		cfg.MaxMessageSize = 512 * 1024
	}
	if cfg.PingInterval == 0 {
		cfg.PingInterval = 30
	}

	h := &Hub{
		config:     cfg,
		clients:    make(map[string]*Client),
		rooms:      make(map[string]*Room),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *broadcastMsg, 256),
	}

	go h.run()
	return h
}

func (h *Hub) OnConnect(fn func(client *Client)) {
	h.onConnect = fn
}

func (h *Hub) OnDisconnect(fn func(client *Client)) {
	h.onDisconnect = fn
}

func (h *Hub) OnMessage(fn func(client *Client, msg *Message)) {
	h.onMessage = fn
}

func (h *Hub) Upgrade() fiber.Handler {
	return websocket.New(func(c *websocket.Conn) {
		clientID := c.Query("id")
		if clientID == "" {
			clientID = fmt.Sprintf("client_%d", time.Now().UnixNano())
		}

		client := &Client{
			ID:     clientID,
			Conn:   c,
			Hub:    h,
			Outbox: make(chan []byte, 256),
			rooms:  make(map[string]bool),
		}

		h.register <- client
		go client.writePump()
		client.readPump()
	})
}

func (h *Hub) UpgradeMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	}
}

func (h *Hub) Broadcast(room string, data []byte) {
	h.broadcast <- &broadcastMsg{room: room, data: data}
}

func (h *Hub) BroadcastExclude(room string, data []byte, excludeClientID string) {
	h.broadcast <- &broadcastMsg{room: room, data: data, exclude: excludeClientID}
}

func (h *Hub) BroadcastAll(data []byte) {
	h.broadcast <- &broadcastMsg{data: data}
}

func (h *Hub) SendTo(clientID string, data []byte) {
	h.mu.RLock()
	client, ok := h.clients[clientID]
	h.mu.RUnlock()

	if ok {
		select {
		case client.Outbox <- data:
		default:
		}
	}
}

func (h *Hub) SendJSON(clientID string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	h.SendTo(clientID, data)
	return nil
}

func (h *Hub) BroadcastJSON(room string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	h.Broadcast(room, data)
	return nil
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) RoomCount(room string) int {
	h.mu.RLock()
	r, ok := h.rooms[room]
	h.mu.RUnlock()
	if !ok {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.mu.Unlock()
			if h.onConnect != nil {
				h.onConnect(client)
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				close(client.Outbox)
				client.leaveAllRooms()
			}
			h.mu.Unlock()
			if h.onDisconnect != nil {
				h.onDisconnect(client)
			}

		case msg := <-h.broadcast:
			if msg.room != "" {
				h.mu.RLock()
				room, ok := h.rooms[msg.room]
				h.mu.RUnlock()
				if ok {
					room.mu.RLock()
					for id, client := range room.clients {
						if id == msg.exclude {
							continue
						}
						select {
						case client.Outbox <- msg.data:
						default:
						}
					}
					room.mu.RUnlock()
				}
			} else {
				h.mu.RLock()
				for id, client := range h.clients {
					if id == msg.exclude {
						continue
					}
					select {
					case client.Outbox <- msg.data:
					default:
					}
				}
				h.mu.RUnlock()
			}
		}
	}
}

func (c *Client) Join(room string) {
	c.Hub.mu.Lock()
	r, ok := c.Hub.rooms[room]
	if !ok {
		r = &Room{ID: room, clients: make(map[string]*Client)}
		c.Hub.rooms[room] = r
	}
	c.Hub.mu.Unlock()

	r.mu.Lock()
	r.clients[c.ID] = c
	r.mu.Unlock()

	c.mu.Lock()
	c.rooms[room] = true
	c.mu.Unlock()
}

func (c *Client) Leave(room string) {
	c.Hub.mu.RLock()
	r, ok := c.Hub.rooms[room]
	c.Hub.mu.RUnlock()

	if ok {
		r.mu.Lock()
		delete(r.clients, c.ID)
		r.mu.Unlock()
	}

	c.mu.Lock()
	delete(c.rooms, room)
	c.mu.Unlock()
}

func (c *Client) Send(data []byte) {
	select {
	case c.Outbox <- data:
	default:
	}
}

func (c *Client) SendJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.Send(data)
	return nil
}

func (c *Client) leaveAllRooms() {
	c.mu.RLock()
	rooms := make([]string, 0, len(c.rooms))
	for room := range c.rooms {
		rooms = append(rooms, room)
	}
	c.mu.RUnlock()

	for _, room := range rooms {
		c.Leave(room)
	}
}

func (c *Client) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(c.Hub.config.MaxMessageSize)

	for {
		_, data, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		if c.Hub.onMessage != nil {
			var msg Message
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}

			switch msg.Type {
			case "join":
				c.Join(msg.Room)
			case "leave":
				c.Leave(msg.Room)
			default:
				c.Hub.onMessage(c, &msg)
			}
		}
	}
}

func (c *Client) writePump() {
	pingInterval := time.Duration(c.Hub.config.PingInterval) * time.Second
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Outbox:
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("  ws: ping failed for client %s: %v", c.ID, err)
				return
			}
		}
	}
}
