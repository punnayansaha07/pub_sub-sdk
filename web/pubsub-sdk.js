/**
 * PubSub JavaScript SDK
 * A lightweight WebSocket client for the PubSub messaging system
 * 
 * @example
 * const client = new PubSubClient('ws://localhost:8080/ws');
 * 
 * client.on('connected', () => {
 *   client.subscribe('my-topic');
 * });
 * 
 * client.on('message', (data) => {
 *   console.log('Received:', data);
 * });
 * 
 * client.connect();
 * client.publish('my-topic', { hello: 'world' });
 */

class PubSubClient {
    /**
     * Create a new PubSub client
     * @param {string} url - WebSocket server URL (e.g., 'ws://localhost:8080/ws')
     * @param {Object} options - Configuration options
     * @param {boolean} options.autoReconnect - Automatically reconnect on disconnect (default: true)
     * @param {number} options.reconnectInterval - Reconnection interval in ms (default: 3000)
     * @param {number} options.maxReconnectAttempts - Max reconnection attempts (default: 5)
     */
    constructor(url, options = {}) {
        this.url = url;
        this.ws = null;
        this.connected = false;
        this.reconnectAttempts = 0;
        
        // Options
        this.autoReconnect = options.autoReconnect !== false;
        this.reconnectInterval = options.reconnectInterval || 3000;
        this.maxReconnectAttempts = options.maxReconnectAttempts || 5;
        
        // Event handlers
        this.eventHandlers = {
            connected: [],
            disconnected: [],
            message: [],
            ack: [],
            error: [],
            subscribed: [],
            unsubscribed: [],
            published: []
        };
        
        // Subscriptions tracking
        this.subscriptions = new Set();
        this.messageQueue = [];
    }

    /**
     * Register an event handler
     * @param {string} event - Event name (connected, disconnected, message, ack, error, subscribed, unsubscribed, published)
     * @param {Function} handler - Event handler function
     * @returns {PubSubClient} - Returns this for chaining
     */
    on(event, handler) {
        if (this.eventHandlers[event]) {
            this.eventHandlers[event].push(handler);
        }
        return this;
    }

    /**
     * Remove an event handler
     * @param {string} event - Event name
     * @param {Function} handler - Event handler function to remove
     * @returns {PubSubClient} - Returns this for chaining
     */
    off(event, handler) {
        if (this.eventHandlers[event]) {
            this.eventHandlers[event] = this.eventHandlers[event].filter(h => h !== handler);
        }
        return this;
    }

    /**
     * Emit an event to all registered handlers
     * @private
     */
    _emit(event, data) {
        if (this.eventHandlers[event]) {
            this.eventHandlers[event].forEach(handler => handler(data));
        }
    }

    /**
     * Connect to the WebSocket server
     * @returns {Promise} - Resolves when connected
     */
    connect() {
        return new Promise((resolve, reject) => {
            try {
                this.ws = new WebSocket(this.url);

                this.ws.onopen = () => {
                    this.connected = true;
                    this.reconnectAttempts = 0;
                    this._emit('connected', { url: this.url });
                    
                    // Process queued messages
                    this._processQueue();
                    
                    resolve();
                };

                this.ws.onmessage = (event) => {
                    this._handleMessage(event);
                };

                this.ws.onerror = (error) => {
                    this._emit('error', { error, type: 'connection' });
                    reject(error);
                };

                this.ws.onclose = () => {
                    this.connected = false;
                    this._emit('disconnected', { 
                        reconnectAttempts: this.reconnectAttempts,
                        willReconnect: this.autoReconnect && this.reconnectAttempts < this.maxReconnectAttempts
                    });
                    
                    // Auto-reconnect
                    if (this.autoReconnect && this.reconnectAttempts < this.maxReconnectAttempts) {
                        this.reconnectAttempts++;
                        setTimeout(() => this.connect(), this.reconnectInterval);
                    }
                };
            } catch (error) {
                reject(error);
            }
        });
    }

    /**
     * Disconnect from the server
     */
    disconnect() {
        this.autoReconnect = false;
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
        this.connected = false;
        this.subscriptions.clear();
    }

    /**
     * Subscribe to a topic
     * @param {string} topic - Topic name
     * @param {Object} options - Subscription options
     * @param {number} options.lastN - Retrieve last N messages (default: 0)
     * @returns {Promise} - Resolves when subscription is acknowledged
     */
    subscribe(topic, options = {}) {
        return new Promise((resolve, reject) => {
            const requestId = this._generateRequestId('sub');
            
            const message = {
                type: 'subscribe',
                topic: topic,
                request_id: requestId,
                last_n: options.lastN || 0
            };

            // Wait for ACK
            const ackHandler = (data) => {
                if (data.request_id === requestId) {
                    this.off('ack', ackHandler);
                    this.subscriptions.add(topic);
                    this._emit('subscribed', { topic });
                    resolve({ topic });
                }
            };

            const errorHandler = (data) => {
                if (data.request_id === requestId) {
                    this.off('error', errorHandler);
                    reject(data);
                }
            };

            this.on('ack', ackHandler);
            this.on('error', errorHandler);

            this._send(message);
        });
    }

    /**
     * Unsubscribe from a topic
     * @param {string} topic - Topic name
     * @returns {Promise} - Resolves when unsubscription is acknowledged
     */
    unsubscribe(topic) {
        return new Promise((resolve, reject) => {
            const requestId = this._generateRequestId('unsub');
            
            const message = {
                type: 'unsubscribe',
                topic: topic,
                request_id: requestId
            };

            // Wait for ACK
            const ackHandler = (data) => {
                if (data.request_id === requestId) {
                    this.off('ack', ackHandler);
                    this.subscriptions.delete(topic);
                    this._emit('unsubscribed', { topic });
                    resolve({ topic });
                }
            };

            const errorHandler = (data) => {
                if (data.request_id === requestId) {
                    this.off('error', errorHandler);
                    reject(data);
                }
            };

            this.on('ack', ackHandler);
            this.on('error', errorHandler);

            this._send(message);
        });
    }

    /**
     * Publish a message to a topic
     * @param {string} topic - Topic name
     * @param {Object} payload - Message payload (will be JSON serialized)
     * @param {Object} options - Publish options
     * @param {string} options.messageId - Custom message ID (optional)
     * @returns {Promise} - Resolves when publish is acknowledged
     */
    publish(topic, payload, options = {}) {
        return new Promise((resolve, reject) => {
            const requestId = this._generateRequestId('pub');
            
            const message = {
                type: 'publish',
                topic: topic,
                request_id: requestId,
                message: {
                    id: options.messageId,
                    payload: payload
                }
            };

            // Wait for ACK
            const ackHandler = (data) => {
                if (data.request_id === requestId) {
                    this.off('ack', ackHandler);
                    this._emit('published', { topic, payload });
                    resolve({ topic, payload });
                }
            };

            const errorHandler = (data) => {
                if (data.request_id === requestId) {
                    this.off('error', errorHandler);
                    reject(data);
                }
            };

            this.on('ack', ackHandler);
            this.on('error', errorHandler);

            this._send(message);
        });
    }

    /**
     * Send a ping to the server
     * @returns {Promise} - Resolves when pong is received
     */
    ping() {
        return new Promise((resolve) => {
            const requestId = this._generateRequestId('ping');
            
            const message = {
                type: 'ping',
                request_id: requestId
            };

            // Wait for pong (treated as ACK)
            const pongHandler = (data) => {
                if (data.request_id === requestId) {
                    this.off('ack', pongHandler);
                    resolve({ latency: Date.now() - parseInt(requestId.split('-')[1]) });
                }
            };

            this.on('ack', pongHandler);
            this._send(message);
        });
    }

    /**
     * Get current connection status
     * @returns {Object} - Connection status
     */
    getStatus() {
        return {
            connected: this.connected,
            url: this.url,
            subscriptions: Array.from(this.subscriptions),
            reconnectAttempts: this.reconnectAttempts,
            queuedMessages: this.messageQueue.length
        };
    }

    /**
     * Handle incoming WebSocket message
     * @private
     */
    _handleMessage(event) {
        try {
            const data = JSON.parse(event.data);
            
            switch (data.type) {
                case 'message':
                    this._emit('message', {
                        topic: data.topic,
                        message: data.message,
                        timestamp: data.ts
                    });
                    break;
                    
                case 'ack':
                case 'pong':
                    this._emit('ack', {
                        request_id: data.request_id,
                        info: data.info,
                        topic: data.topic
                    });
                    break;
                    
                case 'error':
                    this._emit('error', {
                        request_id: data.request_id,
                        error: data.error,
                        type: 'server'
                    });
                    break;
                    
                case 'info':
                    this._emit('info', data);
                    break;
            }
        } catch (error) {
            this._emit('error', { error, type: 'parse' });
        }
    }

    /**
     * Send a message to the server
     * @private
     */
    _send(message) {
        if (this.connected && this.ws) {
            this.ws.send(JSON.stringify(message));
        } else {
            // Queue message for sending when connected
            this.messageQueue.push(message);
        }
    }

    /**
     * Process queued messages
     * @private
     */
    _processQueue() {
        while (this.messageQueue.length > 0) {
            const message = this.messageQueue.shift();
            this._send(message);
        }
    }

    /**
     * Generate a unique request ID
     * @private
     */
    _generateRequestId(prefix) {
        return `${prefix}-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
    }
}

// Export for different module systems
if (typeof module !== 'undefined' && module.exports) {
    module.exports = PubSubClient;
}
if (typeof window !== 'undefined') {
    window.PubSubClient = PubSubClient;
}
