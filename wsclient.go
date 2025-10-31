package microweb

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// WsClientContext is passed to WebSocket client handlers
type WsClientContext struct {
	Event string // "open", "close", "error", "message", "reconnecting"
	Data  WsData
	Error error
}

// WsClientHandler is the message handler function for client
type WsClientHandler func(ctx *WsClientContext) WsData

// WsClientOptions configures the WebSocket client
type WsClientOptions struct {
	URL               string
	ReconnectInterval time.Duration
	PingInterval      time.Duration
	WriteWait         time.Duration
	ReadWait          time.Duration
	EnablePing        bool
	Handler           WsClientHandler
}

// DefaultWsClientOptions returns default client options
func DefaultWsClientOptions(url string, handler WsClientHandler) *WsClientOptions {
	return &WsClientOptions{
		URL:               url,
		ReconnectInterval: 5 * time.Second,
		PingInterval:      30 * time.Second,
		WriteWait:         10 * time.Second,
		ReadWait:          90 * time.Second, // 3x ping interval for safety
		EnablePing:        true,             // Ping/pong enabled by default
		Handler:           handler,
	}
}

// WsClient represents a WebSocket client with auto-reconnect
type WsClient struct {
	conn        *websocket.Conn
	sendChan    chan []byte
	options     *WsClientOptions
	isConnected int32 // atomic
	isRunning   int32 // atomic
	mu          sync.RWMutex
}

// NewWsClient creates a new WebSocket client
func NewWsClient(options *WsClientOptions) *WsClient {
	return &WsClient{
		sendChan:  make(chan []byte, 100),
		options:   options,
		isRunning: 1,
	}
}

// Send sends data to the WebSocket server
func (c *WsClient) Send(data interface{}) {
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

	if atomic.LoadInt32(&c.isRunning) == 1 {
		select {
		case c.sendChan <- message:
		default:
			log.Println("WsClient: send channel full, message dropped")
		}
	}
}

// Close gracefully closes the WebSocket client
func (c *WsClient) Close() {
	atomic.StoreInt32(&c.isRunning, 0)

	c.mu.Lock()
	if c.conn != nil {
		c.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		c.conn.Close()
	}
	c.mu.Unlock()

	close(c.sendChan)
}

// IsConnected returns true if the client is connected
func (c *WsClient) IsConnected() bool {
	return atomic.LoadInt32(&c.isConnected) == 1
}

// Connect establishes WebSocket connection with infinite auto-reconnect
// This will run forever and never give up, ensuring zero-maintenance operation
func (c *WsClient) Connect(ctx context.Context) {
	attemptCount := 0
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Only stop if explicitly cancelled via context
			c.handleClose()
			return

		case <-ticker.C:
			if atomic.LoadInt32(&c.isRunning) != 1 {
				// Only stop if explicitly closed
				c.handleClose()
				return
			}

			// Skip if already connected
			if c.IsConnected() {
				continue
			}

			// Trigger reconnecting event
			if attemptCount == 0 {
				c.handleReconnecting()
			}

			// Attempt connection - never give up, always retry
			if err := c.dial(); err != nil {
				attemptCount++
				log.Printf("WsClient: reconnect attempt %d failed: %v, retrying in %v",
					attemptCount, err, c.options.ReconnectInterval)

				// Wait before next retry, then continue forever
				time.Sleep(c.options.ReconnectInterval)
				continue
			}

			// Connection successful
			log.Printf("WsClient: connected successfully after %d attempts", attemptCount)
			attemptCount = 0

			// Handle open event
			c.handleOpen()

			// Start read/write loops - blocks until disconnected
			c.run()

			// Connection lost, log and retry immediately
			atomic.StoreInt32(&c.isConnected, 0)
			log.Println("WsClient: connection lost, reconnecting...")
		}
	}
}

// dial establishes the WebSocket connection
func (c *WsClient) dial() error {
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	conn, _, err := dialer.Dial(c.options.URL, nil)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	atomic.StoreInt32(&c.isConnected, 1)
	return nil
}

// run starts the read and write loops
func (c *WsClient) run() {
	var wg sync.WaitGroup
	wg.Add(2)

	// Start reader
	go func() {
		defer wg.Done()
		c.readLoop()
	}()

	// Start writer
	go func() {
		defer wg.Done()
		c.writeLoop()
	}()

	// Wait for both to finish
	wg.Wait()
}

// readLoop reads messages from the WebSocket
func (c *WsClient) readLoop() {
	defer func() {
		atomic.StoreInt32(&c.isConnected, 0)
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
		}
		c.mu.Unlock()
	}()

	// Set up pong handler to reset read deadline if ping is enabled
	// This is how we detect if connection is alive
	if c.options.EnablePing {
		c.conn.SetReadDeadline(time.Now().Add(c.options.ReadWait))
		c.conn.SetPongHandler(func(string) error {
			c.conn.SetReadDeadline(time.Now().Add(c.options.ReadWait))
			return nil
		})
	}

	for atomic.LoadInt32(&c.isConnected) == 1 {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			// ANY error from Gorilla WebSocket means connection is dead
			// Cannot continue reading - must exit and reconnect
			// The ping/pong mechanism will detect truly dead connections
			log.Printf("WsClient: read error: %v, reconnecting...", err)
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure) {
				c.handleError(err)
			}
			return
		}

		// Reset deadline on every successful read (if ping enabled)
		if c.options.EnablePing {
			c.conn.SetReadDeadline(time.Now().Add(c.options.ReadWait))
		}

		// Parse message
		data := NewWsData(message)

		// Call handler
		if c.options.Handler != nil {
			ctx := &WsClientContext{
				Event: "message",
				Data:  data,
			}
			reply := c.options.Handler(ctx)

			// Send reply if not nil
			if reply != nil {
				c.Send(reply)
			}
		}
	}
}

// writeLoop writes messages to the WebSocket and keeps connection alive with pings
func (c *WsClient) writeLoop() {
	ticker := time.NewTicker(c.options.PingInterval)
	defer func() {
		ticker.Stop()
		atomic.StoreInt32(&c.isConnected, 0)
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
		}
		c.mu.Unlock()
	}()

	for atomic.LoadInt32(&c.isConnected) == 1 {
		select {
		case message, ok := <-c.sendChan:
			c.conn.SetWriteDeadline(time.Now().Add(c.options.WriteWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("WsClient: write error: %v, reconnecting...", err)
				return
			}

		case <-ticker.C:
			// Send ping to keep connection alive - prevents idle disconnect
			if c.options.EnablePing {
				c.conn.SetWriteDeadline(time.Now().Add(c.options.WriteWait))
				if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					log.Printf("WsClient: ping error: %v, reconnecting...", err)
					return
				}
			}
		}
	}
}

// handleOpen triggers the open event
func (c *WsClient) handleOpen() {
	if c.options.Handler != nil {
		ctx := &WsClientContext{
			Event: "open",
			Data:  make(WsData),
		}
		c.options.Handler(ctx)
	}
}

// handleClose triggers the close event
func (c *WsClient) handleClose() {
	if c.options.Handler != nil {
		ctx := &WsClientContext{
			Event: "close",
			Data:  make(WsData),
		}
		c.options.Handler(ctx)
	}
}

// handleError triggers the error event
func (c *WsClient) handleError(err error) {
	if c.options.Handler != nil {
		ctx := &WsClientContext{
			Event: "error",
			Data:  make(WsData),
			Error: err,
		}
		c.options.Handler(ctx)
	}
}

// handleReconnecting triggers the reconnecting event
func (c *WsClient) handleReconnecting() {
	if c.options.Handler != nil {
		ctx := &WsClientContext{
			Event: "reconnecting",
			Data:  make(WsData),
		}
		c.options.Handler(ctx)
	}
}
