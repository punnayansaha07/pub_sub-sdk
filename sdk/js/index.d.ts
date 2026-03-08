export interface PubSubClientOptions {
    autoReconnect?: boolean;
    reconnectInterval?: number;
    maxReconnectAttempts?: number;
}

export interface SubscribeOptions {
    lastN?: number;
}

export interface PublishOptions {
    messageId?: string;
}

export interface MessageData {
    topic: string;
    message: {
        id: string;
        payload: any;
        ts: string;
    };
    timestamp: string;
}

export interface AckData {
    request_id: string;
    info?: string;
    topic?: string;
}

export interface ErrorData {
    request_id?: string;
    error: {
        code: string;
        message: string;
    };
    type: 'server' | 'connection' | 'parse';
}

export interface DisconnectedData {
    reconnectAttempts: number;
    willReconnect: boolean;
}

export interface StatusData {
    connected: boolean;
    url: string;
    subscriptions: string[];
    reconnectAttempts: number;
    queuedMessages: number;
}

export type EventHandler<T = any> = (data: T) => void;

export declare class PubSubClient {
    constructor(url: string, options?: PubSubClientOptions);
    
    connect(): Promise<void>;
    disconnect(): void;
    
    subscribe(topic: string, options?: SubscribeOptions): Promise<{ topic: string }>;
    unsubscribe(topic: string): Promise<{ topic: string }>;
    
    publish(topic: string, payload: any, options?: PublishOptions): Promise<{ topic: string; payload: any }>;
    
    ping(): Promise<{ latency: number }>;
    
    getStatus(): StatusData;
    
    on(event: 'connected', handler: EventHandler<{ url: string }>): this;
    on(event: 'disconnected', handler: EventHandler<DisconnectedData>): this;
    on(event: 'message', handler: EventHandler<MessageData>): this;
    on(event: 'ack', handler: EventHandler<AckData>): this;
    on(event: 'error', handler: EventHandler<ErrorData>): this;
    on(event: 'subscribed', handler: EventHandler<{ topic: string }>): this;
    on(event: 'unsubscribed', handler: EventHandler<{ topic: string }>): this;
    on(event: 'published', handler: EventHandler<{ topic: string; payload: any }>): this;
    
    off(event: string, handler: EventHandler): this;
}

export default PubSubClient;
