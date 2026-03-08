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

---

# Overview

This project implements a real-time **publish/subscribe messaging system** using Go.

The system supports **WebSocket clients**, **Redis-backed persistence**, and a **sharded broker architecture** designed to handle high concurrency with predictable latency.

It can be used in applications requiring real-time data distribution such as:

- dashboards
- chat systems
- notification services
- event pipelines
- IoT telemetry streams

---

# Why This Project Exists

Many messaging systems such as **Apache Kafka**, **NATS**, or **RabbitMQ** are designed for large distributed infrastructures.

While powerful, they often introduce operational overhead for applications that only require:

- real-time message delivery
- topic-based fanout
- simple deployment
- WebSocket client support

This project explores a simpler architecture focused on:

- WebSocket-based messaging
- in-memory topic routing
- Redis-backed persistence
- sharded broker design for concurrency

The goal is to build a **high-throughput real-time messaging layer that is easy to deploy and reason about**.

---

# Features

- WebSocket-based real-time messaging
- Redis-backed message persistence
- Broker sharding to reduce lock contention
- Topic-based fanout messaging
- Message replay (last N messages)
- Slow consumer protection
- REST API for topic management
- Health checks and runtime statistics
- Docker-based deployment
- Client SDKs for JavaScript and Go

---

# Architecture Overview

```
Broker
 ├── Shard (32)
 │    └── Topic
 │         └── Subscribers
```

Topics are distributed across shards using a hash of the topic name.

```
shard = hash(topic) % 32
```

This allows operations on different topics to execute in parallel and minimizes lock contention.

---

# Message Flow

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

### Flow

1. A publisher sends a message via WebSocket.
2. The broker hashes the topic name to determine the shard.
3. The topic stores the message in Redis.
4. The message is fanned out to subscribers.
5. Each subscriber receives messages through its bounded queue.

---

# System Design Decisions

## Sharded Broker

A single global lock across all topics would create contention under load.

Topics are distributed across **32 shards**, each protected by its own lock.  
This allows independent operations on different topics.

---

## Bounded Subscriber Queues

Each subscriber maintains a fixed-size message queue.

This prevents slow consumers from causing unbounded memory growth.

If the queue becomes full, slow-consumer protection is triggered.

---

## Slow Consumer Protection

If a subscriber cannot keep up with message delivery:

1. An error message is sent (best effort)
2. The WebSocket connection is closed
3. The subscriber is removed from the topic

This ensures publishers never block due to slow subscribers.

---

## Message Replay

Each topic maintains recent messages in Redis.

When subscribing, a client may request the **last N messages**.

This allows new subscribers to receive recent events without querying storage manually.

---

# Concurrency Model

The system relies heavily on Go concurrency primitives.

## Shard Locks

Each shard protects its topic map using `sync.RWMutex`.

This allows:

- concurrent reads for topic lookup
- exclusive writes for topic creation or deletion

---

## Topic Locks

Each topic maintains its own lock protecting its subscriber map.

Operations on different topics therefore do not block each other.

---

## Subscriber Goroutines

Each subscriber runs a dedicated goroutine responsible for:

- reading messages from its queue
- writing messages to the WebSocket connection

This isolates network latency from the publish path.

---

## Non-Blocking Fanout

Fanout uses non-blocking channel operations:

```go
select {
case subscriber.queue <- msg:
    // success
default:
    disconnectSlowConsumer()
}
```

Publishers never block due to slow subscribers.

---

# Project Structure

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
│       └── protocol.go
├── sdk/
│   ├── js/
│   └── go/
```

---

# Running the Server

## Using Docker

```bash
docker-compose up -d
```

or

```bash
docker build -t pubsub-server .
docker run -p 8080:8080 pubsub-server
```

---

## Running from Source

### Requirements

- Go 1.23+
- Redis

### Start Redis

```bash
docker run -d -p 6379:6379 redis:7-alpine
```

### Run Server

```bash
export REDIS_ADDR=localhost:6379
go run ./cmd/server/main.go
```

Server runs at:

```
http://localhost:8080
```

---

# REST API

### Create Topic

```
POST /topics
```

Example

```bash
curl -X POST http://localhost:8080/topics \
  -H "Content-Type: application/json" \
  -d '{"name":"orders"}'
```

### List Topics

```
GET /topics
```

### Delete Topic

```
DELETE /topics/{name}
```

### System Stats

```
GET /stats
```

### Health Check

```
GET /health
```

---

# WebSocket Protocol

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

### Delivered Message

```json
{
  "type": "message",
  "topic": "orders",
  "message": {
    "payload": { "order_id": "12345" }
  }
}
```

---

# System Limits

| Component | Limit |
|-----------|------|
| Shards | 32 |
| Topics | 10,000 |
| Subscribers per topic | 50,000 |
| Subscriber queue size | 100 |
| Replay buffer | 100 messages |

---

# Benchmarks

**Test environment**

- 8 vCPU
- 16 GB RAM
- Redis 7
- Go 1.23

**Approximate results**

| Metric | Result |
|------|------|
| Concurrent subscribers | 50,000+ |
| Publish throughput | ~100k messages/sec |
| Fanout throughput | ~500k deliveries/sec |
| Publish latency | ~20µs |

---

# Comparison With Other Systems

| System | Focus | Complexity | Typical Use |
|------|------|------|------|
| Kafka | Distributed log | High | Data pipelines |
| NATS | Messaging infrastructure | Medium | Microservices |
| Redis Pub/Sub | Simple pub/sub | Low | Small real-time systems |
| This project | WebSocket fanout broker | Low | Real-time apps |

---

# Use Cases

- real-time dashboards
- chat systems
- notification delivery
- IoT telemetry
- live event feeds
- multiplayer game state updates

---

# Client SDKs

### JavaScript / TypeScript

```bash
npm install @pubsub/client
```

See `sdk/js/README.md`.

---

### Go

```bash
go get github.com/punnayan-josys/pubsub-client-go/client
```

See `sdk/go/README.md`.

---

# Contributing

Contributions are welcome.

Basic workflow

```bash
git checkout -b feature/my-feature
git commit -m "add feature"
git push
```

Run tests

```bash
go test ./...
```

---

# License

MIT License

---

<p align="center">
⭐ Star this project on <a href="https://github.com/punnayan-josys/PUB-SUM">GitHub</a>
</p>
