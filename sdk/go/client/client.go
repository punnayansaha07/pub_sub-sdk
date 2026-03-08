package client

import (
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ClientOptions configures the PubSub client
type ClientOptions struct {
	AutoReconnect        bool
	ReconnectInterval    time.Duration
	MaxReconnectAttempts int
}

// DefaultOptions returns default client options
func DefaultOptions() ClientOptions {
	return ClientOptions{
		AutoReconnect:        true,
		ReconnectInterval:    3 * time.Second,
		MaxReconnectAttempts: 5,
	}
}

// MessageData represents a received message
type MessageData struct {
	Topic     string    `json:"topic"`
	Message   Message   `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Message represents the message structure
type Message struct {
	ID        string                 `json:"id"`
	Payload   map[string]interface{} `json:"payload"`
	Timestamp time.Time              `json:"ts"`
}

// EventHandler is a function type for handling events
type EventHandler func(data interface{})

// Client represents a PubSub WebSocket client
type Client struct {
	url               string
	conn              *websocket.Conn
	connected         bool
	reconnectAttempts int
	options           ClientOptions

	eventHandlers map[string][]EventHandler
	subscriptions map[string]bool
	messageQueue  []interface{}

	mu        sync.RWMutex
	closeChan chan struct{}
}

// NewClient creates a new PubSub client
func NewClient(url string, options ...ClientOptions) *Client {
	opts := DefaultOptions()
	if len(options) > 0 {
		opts = options[0]
	}

	return &Client{
		url:           url,
		options:       opts,
		eventHandlers: make(map[string][]EventHandler),
		subscriptions: make(map[string]bool),
		messageQueue:  make([]interface{}, 0),
		closeChan:     make(chan struct{}),
	}
}

// On registers an event handler
func (c *Client) On(event string, handler EventHandler) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.eventHandlers[event] = append(c.eventHandlers[event], handler)
	return c
}

// Off removes an event handler
func (c *Client) Off(event string) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.eventHandlers, event)
	return c
}

// emit triggers event handlers
func (c *Client) emit(event string, data interface{}) {
	c.mu.RLock()
	handlers := c.eventHandlers[event]
	c.mu.RUnlock()

	for _, handler := range handlers {
		go handler(data)
	}
}

// Connect establishes a WebSocket connection
func (c *Client) Connect() error {
	conn, _, err := websocket.DefaultDialer.Dial(c.url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.reconnectAttempts = 0
	c.mu.Unlock()

	c.emit("connected", map[string]interface{}{"url": c.url})

	// Process queued messages
	c.processQueue()

	// Start reading messages
	go c.readLoop()

	return nil
}

// Disconnect closes the WebSocket connection
func (c *Client) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.options.AutoReconnect = false
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.connected = false
	c.subscriptions = make(map[string]bool)
	close(c.closeChan)
}

// Subscribe subscribes to a topic
func (c *Client) Subscribe(topic string, lastN ...int) error {
	requestID := generateRequestID("sub")

	msg := map[string]interface{}{
		"type":       "subscribe",
		"topic":      topic,
		"request_id": requestID,
	}

	if len(lastN) > 0 {
		msg["last_n"] = lastN[0]
	}

	if err := c.send(msg); err != nil {
		return err
	}

	c.mu.Lock()
	c.subscriptions[topic] = true
	c.mu.Unlock()

	c.emit("subscribed", map[string]interface{}{"topic": topic})
	return nil
}

// Unsubscribe unsubscribes from a topic
func (c *Client) Unsubscribe(topic string) error {
	requestID := generateRequestID("unsub")

	msg := map[string]interface{}{
		"type":       "unsubscribe",
		"topic":      topic,
		"request_id": requestID,
	}

	if err := c.send(msg); err != nil {
		return err
	}

	c.mu.Lock()
	delete(c.subscriptions, topic)
	c.mu.Unlock()

	c.emit("unsubscribed", map[string]interface{}{"topic": topic})
	return nil
}

// Publish publishes a message to a topic
func (c *Client) Publish(topic string, payload map[string]interface{}) error {
	requestID := generateRequestID("pub")

	msg := map[string]interface{}{
		"type":       "publish",
		"topic":      topic,
		"request_id": requestID,
		"message": map[string]interface{}{
			"payload": payload,
		},
	}

	if err := c.send(msg); err != nil {
		return err
	}

	c.emit("published", map[string]interface{}{"topic": topic, "payload": payload})
	return nil
}

// Ping sends a ping to the server
func (c *Client) Ping() (time.Duration, error) {
	requestID := generateRequestID("ping")
	start := time.Now()

	msg := map[string]interface{}{
		"type":       "ping",
		"request_id": requestID,
	}

	if err := c.send(msg); err != nil {
		return 0, err
	}

	return time.Since(start), nil
}

// GetStatus returns the current connection status
func (c *Client) GetStatus() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	subs := make([]string, 0, len(c.subscriptions))
	for topic := range c.subscriptions {
		subs = append(subs, topic)
	}

	return map[string]interface{}{
		"connected":         c.connected,
		"url":               c.url,
		"subscriptions":     subs,
		"reconnectAttempts": c.reconnectAttempts,
		"queuedMessages":    len(c.messageQueue),
	}
}

// send sends a message to the server
func (c *Client) send(msg interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.conn == nil {
		c.messageQueue = append(c.messageQueue, msg)
		return nil
	}

	return c.conn.WriteJSON(msg)
}

// processQueue sends queued messages
func (c *Client) processQueue() {
	c.mu.Lock()
	queue := c.messageQueue
	c.messageQueue = make([]interface{}, 0)
	c.mu.Unlock()

	for _, msg := range queue {
		c.send(msg)
	}
}

// readLoop reads messages from the WebSocket
func (c *Client) readLoop() {
	defer func() {
		c.mu.Lock()
		c.connected = false
		c.mu.Unlock()

		c.emit("disconnected", map[string]interface{}{
			"reconnectAttempts": c.reconnectAttempts,
			"willReconnect":     c.options.AutoReconnect && c.reconnectAttempts < c.options.MaxReconnectAttempts,
		})

		// Auto-reconnect
		if c.options.AutoReconnect && c.reconnectAttempts < c.options.MaxReconnectAttempts {
			c.reconnectAttempts++
			time.Sleep(c.options.ReconnectInterval)
			c.Connect()
		}
	}()

	for {
		var msg map[string]interface{}
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.emit("error", map[string]interface{}{"error": err, "type": "connection"})
			}
			return
		}

		c.handleMessage(msg)
	}
}

// handleMessage processes incoming messages
func (c *Client) handleMessage(msg map[string]interface{}) {
	msgType, ok := msg["type"].(string)
	if !ok {
		return
	}

	switch msgType {
	case "message":
		c.emit("message", msg)

	case "ack", "pong":
		c.emit("ack", msg)

	case "error":
		c.emit("error", map[string]interface{}{
			"request_id": msg["request_id"],
			"error":      msg["error"],
			"type":       "server",
		})

	case "info":
		c.emit("info", msg)
	}
}

// generateRequestID generates a unique request ID
func generateRequestID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}
