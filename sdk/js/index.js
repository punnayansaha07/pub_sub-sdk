/**
 * PubSub JavaScript/TypeScript Client SDK
 */

class PubSubClient {
    constructor(url, options = {}) {
        this.url = url;
        this.ws = null;
        this.connected = false;
        this.reconnectAttempts = 0;
        
        this.autoReconnect = options.autoReconnect !== false;
        this.reconnectInterval = options.reconnectInterval || 3000;
        this.maxReconnectAttempts = options.maxReconnectAttempts || 5;
        
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
        
        this.subscriptions = new Set();
        this.messageQueue = [];
    }

    on(event, handler) {
        if (this.eventHandlers[event]) {
            this.eventHandlers[event].push(handler);
        }
        return this;
    }

    off(event, handler) {
        if (this.eventHandlers[event]) {
            this.eventHandlers[event] = this.eventHandlers[event].filter(h => h !== handler);
        }
        return this;
    }

    _emit(event, data) {
        if (this.eventHandlers[event]) {
            this.eventHandlers[event].forEach(handler => handler(data));
        }
    }

    connect() {
        return new Promise((resolve, reject) => {
            try {
                const WebSocket = typeof window !== 'undefined' ? window.WebSocket : require('ws');
                this.ws = new WebSocket(this.url);

                this.ws.onopen = () => {
                    this.connected = true;
                    this.reconnectAttempts = 0;
                    this._emit('connected', { url: this.url });
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

    disconnect() {
        this.autoReconnect = false;
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
        this.connected = false;
        this.subscriptions.clear();
    }

    subscribe(topic, options = {}) {
        return new Promise((resolve, reject) => {
            const requestId = this._generateRequestId('sub');
            
            const message = {
                type: 'subscribe',
                topic: topic,
                request_id: requestId,
                last_n: options.lastN || 0
            };

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

    unsubscribe(topic) {
        return new Promise((resolve, reject) => {
            const requestId = this._generateRequestId('unsub');
            
            const message = {
                type: 'unsubscribe',
                topic: topic,
                request_id: requestId
            };

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

    ping() {
        return new Promise((resolve) => {
            const requestId = this._generateRequestId('ping');
            const startTime = Date.now();
            
            const message = {
                type: 'ping',
                request_id: requestId
            };

            const pongHandler = (data) => {
                if (data.request_id === requestId) {
                    this.off('ack', pongHandler);
                    resolve({ latency: Date.now() - startTime });
                }
            };

            this.on('ack', pongHandler);
            this._send(message);
        });
    }

    getStatus() {
        return {
            connected: this.connected,
            url: this.url,
            subscriptions: Array.from(this.subscriptions),
            reconnectAttempts: this.reconnectAttempts,
            queuedMessages: this.messageQueue.length
        };
    }

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

    _send(message) {
        if (this.connected && this.ws) {
            this.ws.send(JSON.stringify(message));
        } else {
            this.messageQueue.push(message);
        }
    }

    _processQueue() {
        while (this.messageQueue.length > 0) {
            const message = this.messageQueue.shift();
            this._send(message);
        }
    }

    _generateRequestId(prefix) {
        return `${prefix}-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
    }
}

module.exports = PubSubClient;
