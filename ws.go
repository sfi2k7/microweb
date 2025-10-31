package microweb

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Global Hub instance
var Hub *WsHub

// WsConfig configures WebSocket behavior
type WsConfig struct {
	PingInterval    time.Duration
	PongWait        time.Duration
	WriteWait       time.Duration
	MaxMessageSize  int64
	ReadBufferSize  int
	WriteBufferSize int
}

// DefaultWsConfig returns default WebSocket configuration
func DefaultWsConfig() *WsConfig {
	return &WsConfig{
		PingInterval:    30 * time.Second,
		PongWait:        60 * time.Second,
		WriteWait:       10 * time.Second,
		MaxMessageSize:  512 * 1024, // 512 KB
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
}

// WsData represents dynamic JSON data with type-safe getters
type WsData map[string]interface{}

// NewWsData creates WsData from JSON bytes
func NewWsData(jsonBytes []byte) WsData {
	var data map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return make(WsData)
	}
	return WsData(data)
}

// NewWsDataFromMap creates WsData from a map
func NewWsDataFromMap(data map[string]interface{}) WsData {
	return WsData(data)
}

// String returns string value for key
func (w WsData) String(key string) string {
	if v, ok := w[key]; ok {
		if str, ok := v.(string); ok {
			return str
		}
	}
	return ""
}

// Int returns int value for key
func (w WsData) Int(key string) int {
	if v, ok := w[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		case int64:
			return int(val)
		}
	}
	return 0
}

// Float returns float64 value for key
func (w WsData) Float(key string) float64 {
	if v, ok := w[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return 0.0
}

// Bool returns bool value for key
func (w WsData) Bool(key string) bool {
	if v, ok := w[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// Get returns raw interface{} value for key
func (w WsData) Get(key string) interface{} {
	return w[key]
}

// Has checks if key exists
func (w WsData) Has(key string) bool {
	_, ok := w[key]
	return ok
}

// Set sets a key-value pair
func (w WsData) Set(key string, value interface{}) {
	w[key] = value
}

// Raw returns the underlying map
func (w WsData) Raw() map[string]interface{} {
	return w
}

// ToJSON converts WsData to JSON bytes
func (w WsData) ToJSON() []byte {
	data, _ := json.Marshal(w)
	return data
}

// EventHandler handles WebSocket lifecycle events
type EventHandler func(ctx *ClientContext)

// WsHandler is the message handler function
type WsHandler func(ctx *ClientContext) WsData

// Client represents a WebSocket client connection
type Client struct {
	Id     string
	conn   *websocket.Conn
	send   chan []byte
	hub    *WsHub
	events map[string][]EventHandler
	mu     sync.RWMutex
}

// On registers an event handler
func (c *Client) On(event string, handler EventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events[event] = append(c.events[event], handler)
}

// emit triggers event handlers
func (c *Client) emit(event string, ctx *ClientContext) {
	c.mu.RLock()
	handlers := c.events[event]
	c.mu.RUnlock()

	for _, handler := range handlers {
		handler(ctx)
	}
}

// Send sends data to this client
func (c *Client) Send(data interface{}) {
	var message []byte
	switch v := data.(type) {
	case []byte:
		message = v
	case string:
		message = []byte(v)
	case WsData:
		message = v.ToJSON()
	default:
		message, _ = json.Marshal(data)
	}

	select {
	case c.send <- message:
	default:
		// Channel full, close connection
		c.hub.unregister <- c
	}
}

// Close closes the client connection
func (c *Client) Close() {
	c.hub.unregister <- c
}

// ClientContext is passed to WebSocket handlers
type ClientContext struct {
	Id     string
	Data   WsData
	client *Client
}

// On registers an event handler for this client
func (ctx *ClientContext) On(event string, handler EventHandler) {
	ctx.client.On(event, handler)
}

// Send sends data to this client
func (ctx *ClientContext) Send(data interface{}) {
	ctx.client.Send(data)
}

// Close closes this client connection
func (ctx *ClientContext) Close() {
	ctx.client.Close()
}

// SendMessage represents a message to send to a specific client
type SendMessage struct {
	ClientId string
	Message  []byte
}

// BroadcastMessage represents a message to broadcast to all clients
type BroadcastMessage struct {
	Message []byte
}

// WsHub manages all WebSocket connections
type WsHub struct {
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan *BroadcastMessage
	sendMsg    chan *SendMessage
	mu         sync.RWMutex
	config     *WsConfig
}

// NewWsHub creates a new WebSocket hub
func NewWsHub(config *WsConfig) *WsHub {
	if config == nil {
		config = DefaultWsConfig()
	}
	return &WsHub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *BroadcastMessage),
		sendMsg:    make(chan *SendMessage),
		config:     config,
	}
}

// Run starts the hub's main loop
func (h *WsHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.Id] = client
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.Id]; ok {
				delete(h.clients, client.Id)
				close(client.send)
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			for _, client := range h.clients {
				select {
				case client.send <- msg.Message:
				default:
					close(client.send)
					delete(h.clients, client.Id)
				}
			}
			h.mu.RUnlock()

		case msg := <-h.sendMsg:
			h.mu.RLock()
			if client, ok := h.clients[msg.ClientId]; ok {
				select {
				case client.send <- msg.Message:
				default:
					close(client.send)
					delete(h.clients, client.Id)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Send sends a message to a specific client
func (h *WsHub) Send(clientId string, message interface{}) {
	var msg []byte
	switch v := message.(type) {
	case []byte:
		msg = v
	case string:
		msg = []byte(v)
	case WsData:
		msg = v.ToJSON()
	default:
		msg, _ = json.Marshal(message)
	}

	h.sendMsg <- &SendMessage{
		ClientId: clientId,
		Message:  msg,
	}
}

// Broadcast sends a message to all connected clients
func (h *WsHub) Broadcast(message interface{}) {
	var msg []byte
	switch v := message.(type) {
	case []byte:
		msg = v
	case string:
		msg = []byte(v)
	case WsData:
		msg = v.ToJSON()
	default:
		msg, _ = json.Marshal(message)
	}

	h.broadcast <- &BroadcastMessage{Message: msg}
}

// Close closes a specific client connection
func (h *WsHub) Close(clientId string) {
	h.mu.RLock()
	client, ok := h.clients[clientId]
	h.mu.RUnlock()

	if ok {
		h.unregister <- client
	}
}

// Count returns the number of connected clients
func (h *WsHub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetClient returns a client by ID
func (h *WsHub) GetClient(clientId string) *Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clients[clientId]
}

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// Ws registers a WebSocket handler
func (r *Router) Ws(path string, handler WsHandler) {
	// Initialize global Hub if not exists
	if Hub == nil {
		Hub = NewWsHub(DefaultWsConfig())
		go Hub.Run()
	}

	r.Get(path, func(ctx *Context) {
		serveWs(Hub, ctx.W, ctx.R, handler)
	})
}

// WebSocketHub returns the global hub
func (r *Router) WebSocketHub() *WsHub {
	return Hub
}

// serveWs handles WebSocket requests
func serveWs(hub *WsHub, w http.ResponseWriter, r *http.Request, handler WsHandler) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Generate unique client ID (UUID without dashes)
	clientId := uuid.New().String()
	clientId = clientId[:8] + clientId[9:13] + clientId[14:18] + clientId[19:23] + clientId[24:]

	client := &Client{
		Id:     clientId,
		conn:   conn,
		send:   make(chan []byte, 256),
		hub:    hub,
		events: make(map[string][]EventHandler),
	}

	hub.register <- client

	// Create context for open event
	ctx := &ClientContext{
		Id:     client.Id,
		Data:   NewWsDataFromMap(make(map[string]interface{})),
		client: client,
	}

	// Emit open event
	client.emit("open", ctx)

	// Start goroutines
	go writePump(client, hub.config)
	go readPump(client, hub.config, handler)
}

// readPump reads messages from the WebSocket connection
func readPump(client *Client, config *WsConfig, handler WsHandler) {
	defer func() {
		client.hub.unregister <- client
		client.conn.Close()
	}()

	client.conn.SetReadDeadline(time.Now().Add(config.PongWait))
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(config.PongWait))
		return nil
	})
	client.conn.SetReadLimit(config.MaxMessageSize)

	for {
		_, message, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// Emit error event
				ctx := &ClientContext{
					Id:     client.Id,
					Data:   NewWsDataFromMap(map[string]interface{}{"error": err.Error()}),
					client: client,
				}
				client.emit("error", ctx)
			}
			// Emit close event
			ctx := &ClientContext{
				Id:     client.Id,
				Data:   NewWsDataFromMap(make(map[string]interface{})),
				client: client,
			}
			client.emit("close", ctx)
			break
		}

		// Parse message as JSON
		wsData := NewWsData(message)

		// Create context
		ctx := &ClientContext{
			Id:     client.Id,
			Data:   wsData,
			client: client,
		}

		// Call handler
		reply := handler(ctx)

		// Send reply if not nil
		if reply != nil {
			client.Send(reply)
		}
	}
}

// writePump writes messages to the WebSocket connection
func writePump(client *Client, config *WsConfig) {
	ticker := time.NewTicker(config.PingInterval)
	defer func() {
		ticker.Stop()
		client.conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			client.conn.SetWriteDeadline(time.Now().Add(config.WriteWait))
			if !ok {
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := client.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to current websocket message
			n := len(client.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-client.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(config.WriteWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
