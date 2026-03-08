# Pub/Sub Messaging System

<p align="center">
  <strong>A lightweight real-time publish/subscribe messaging system written in Go</strong>
</p>

<p align="center">
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.23-blue.svg" alt="Go Version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green.svg" alt="License"></a>
  <a href="https://hub.docker.com/"><img src="https://img.shields.io/badge/docker-ready-blue.svg" alt="Docker"></a>
  <a href="https://github.com/punnayan-josys/PUB-SUM/stargazers"><img src="https://img.shields.io/github/stars/punnayan-josys/PUB-SUM?style=social" alt="Stars"></a>
</p>

## Overview

The system supports WebSocket clients, Redis-backed message persistence, and a sharded broker architecture designed to handle high concurrency with predictable latency.

It is suitable for applications that require real-time data distribution such as dashboards, chat systems, notification services, and event pipelines.

## Why This Project Exists

Most modern messaging systems such as Apache Kafka, NATS, or RabbitMQ are designed for large distributed deployments.

While powerful, they often introduce operational overhead for applications that only require:

- Real-time message delivery
- Topic-based fan-out
- Simple deployment
- WebSocket client support

This project explores a simpler architecture focused on:

- WebSocket-based messaging
- In-memory topic routing
- Redis-backed persistence
- Sharded broker design for concurrency

The goal is to provide a high-throughput real-time messaging layer that is easy to deploy and reason about.

## Features

- **WebSocket-based real-time messaging**
- **Redis-backed message persistence**
- **Broker sharding** to reduce lock contention
- **Topic-based fanout** messaging
- **Message replay** (last N messages)
- **Slow consumer protection**
- **REST API** for topic management
- **Health checks** and runtime statistics
- **Docker-based** deployment
- **Client SDKs** for JavaScript and Go

Architecture Overview
Broker
 ├── Shard (32)
 │    └── Topic
 │         └── Subscribers


Topics are distributed across shards using a hash of the topic name.

## Architecture Overview

```
Broker
 ├── Shard (32)
### Message Flow

```
Publisher
   │
   │ publish(topic, message)
   ▼
WebSocket Handler
   │
   ▼
Broker
   │
   ├── shard = hash(topic) % 32
   ▼
Shard
   │
   ▼
Topic
   │
   ├── store message → Redis
   │
   └── fanout message
          │
          ▼
     Subscribers
```

**Flow:**

1. Publisher sends a message via WebSocket
2. Broker hashes the topic to determine the shard
3. Topic persists the message to Redis
4. Message is fanned out to subscribers
5. Each subscriber receives the message through its bounded queue
Message is fanned out to subscribers.

Each subscriber receives the message through its bounded queue.
## System Design Decisions

### Sharded Broker

A single global lock on all topics would become a bottleneck under load.

Topics are therefore distributed across 32 shards, each protected by its own lock. This allows independent operations on different topics.

### Bounded Subscriber Queues

Each subscriber has a fixed-size message queue.

This prevents slow consumers from causing unbounded memory growth. If the queue becomes full, slow-consumer protection is triggered.

### Slow Consumer Protection

If a subscriber cannot keep up with message delivery:

1. An error message is sent (best effort)
2. The WebSocket connection is closed
3. The subscriber is removed from the topic

This ensures publishers never block due to slow consumers.

### Message Replay

Each topic maintains recent messages in Redis.

When subscribing, a client may request the last N messages. 
This enables late subscribers to receive recent events without querying storage.

Concurrency Model

The system relies heavily on Go concurrency primitives.

Shard Locks

Each shard protects its topic map using sync.RWMutex.

## Concurrency Model

The system relies heavily on Go concurrency primitives.

### Shard Locks

Each shard protects its topic map using `sync.RWMutex`.

This allows:
- Concurrent reads for topic lookup
- Exclusive writes for topic creation and deletion

### Topic Locks

Each topic maintains its own lock to protect the subscriber map.

Operations on different topics do not block each other.

### Subscriber Goroutines

Each subscriber runs a dedicated goroutine responsible for:
- Reading messages from its queue
- Writing messages to the WebSocket connection

This isolates network latency from the publish path.

### Non-Blocking Fanout

Fanout uses non-blocking channel operations:

```go
select {
case subscriber.queue <- msg:
    // Success
default:
    disconnectSlowConsumer()
}
```├── internal
│   ├── broker
│   │   ├── broker.go
│   │   ├── shard.go
│   │   ├── topic.go
│   │   └── subscriber.go
## Project Structure

```
pubsub/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── broker/
│   │   ├── broker.go
│   │   ├── shard.go
│   │   ├── topic.go
│   │   └── subscriber.go
│   ├── storage/
│   │   └── redis.go
│   ├── ws/
│   │   └── handler.go
│   ├── api/
│   │   ├── topics.go
│   │   ├── stats.go
│   │   └── health.go
│   └── model/
│       ├── message.go
## Running the Server

### Using Docker

```bash
docker-compose up -d
```

or

```bash
docker build -t pubsub-server .
docker run -p 8080:8080 pubsub-server
```

### Running from Source

**Requirements:**
- Go 1.23+
- Redis

**Start Redis:**

```bash
docker run -d -p 6379:6379 redis:7-alpine
```

**Run server:**

```bash
export REDIS_ADDR=localhost:6379
go run ./cmd/server/main.go
```
## REST API

### Create topic

```
POST /topics
```

**Example:**

```bash
curl -X POST http://localhost:8080/topics \
  -H "Content-Type: application/json" \
  -d '{"name":"orders"}'
```

### List topics

```
GET /topics
```

### Delete topic

```
DELETE /topics/{name}
```
## WebSocket Protocol

### Connect

```
ws://localhost:8080/ws
```

### Subscribe

```json
{
  "type": "subscribe",
  "topic": "orders",
  "last_n": 10
}
```

### Publish

```json
{
  "type": "publish",
  "topic": "orders",
  "message": {
    "payload": { "order_id": "12345" }
  }
}
```

### Delivered message

```json
{
  "type": "message",
## System Limits

| Component | Limit |
|-----------|-------|
| Shards | 32 |
| Topics | 10,000 |
| Subscribers per topic | 50,000 |
## Benchmarks

**Test environment:**
- 8 vCPU
- 16 GB RAM
- Redis 7
- Go 1.23

**Approximate results:**

| Metric | Result |
|--------|--------|
| Concurrent subscribers | 50,000+ |
| Publish throughput | ~100k messages/sec |
| Fanout throughput | ~500k deliveries/sec |
| Publish latency | ~20µs |
Shards	32
Topics	10,000
Subscribers per topic	50,000
Subscriber queue size	100
Replay buffer	100 messages
Benchmarks
## Comparison With Other Systems

| System | Focus | Complexity | Typical Use |
|--------|-------|------------|-------------|
| Kafka | Distributed log | High | Data pipelines |
| NATS | Messaging infrastructure | Medium | Microservices |
| Redis Pub/Sub | Simple pub/sub | Low | Small real-time systems |
| **This project** | **WebSocket fanout broker** | **Low** | **Real-time apps** |
Go 1.23


Approximate results
## Use Cases

- Real-time dashboards
- Chat systems
- Notification delivery
- IoT telemetry streams
- Live event feeds
- Multiplayer game state updates

## Client SDKs

### JavaScript/TypeScript

```bash
npm install @pubsub/client
```

See [JavaScript SDK Documentation](sdk/js/README.md)

### Go

```bash
go get github.com/punnayan-josys/pubsub-client-go/client
```

See [Go SDK Documentation](sdk/go/README.md)

## Contributing

Contributions are welcome!

**Basic workflow:**

```bash
git checkout -b feature/my-feature
git commit -m "add feature"
git push
```

**Run tests:**

```bash
go test ./...
```

## License

MIT License

---

<p align="center">⭐ Star this project on <a href="https://github.com/punnayan-josys/PUB-SUM">GitHub</a></p>ns are welcome.

Basic workflow

git checkout -b feature/my-feature
git commit -m "add feature"
git push


Run tests

go test ./...

License

MIT License