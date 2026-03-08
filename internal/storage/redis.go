package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"pubsub/internal/model"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStorage handles Redis persistence
type RedisStorage struct {
	client       *redis.Client
	ctx          context.Context
	topicPrefix  string
	replayPrefix string
	replaySize   int
}

// NewRedisStorage creates a new Redis storage instance
func NewRedisStorage(addr string, password string, db int, replaySize int) (*RedisStorage, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx := context.Background()

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Printf("Connected to Redis at %s", addr)

	return &RedisStorage{
		client:       client,
		ctx:          ctx,
		topicPrefix:  "pubsub:topic:",
		replayPrefix: "pubsub:replay:",
		replaySize:   replaySize,
	}, nil
}

// Close closes the Redis connection
func (r *RedisStorage) Close() error {
	return r.client.Close()
}

// SaveMessage saves a message to Redis for replay
func (r *RedisStorage) SaveMessage(topicName string, message model.Message) error {
	key := r.replayPrefix + topicName

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Add to list (LPUSH for newest first)
	if err := r.client.LPush(r.ctx, key, data).Err(); err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}

	// Trim to keep only last N messages
	if err := r.client.LTrim(r.ctx, key, 0, int64(r.replaySize-1)).Err(); err != nil {
		return fmt.Errorf("failed to trim replay buffer: %w", err)
	}

	// Set expiration (7 days)
	r.client.Expire(r.ctx, key, 7*24*time.Hour)

	return nil
}

// GetReplayMessages retrieves last N messages for a topic
func (r *RedisStorage) GetReplayMessages(topicName string, lastN int) ([]model.Message, error) {
	key := r.replayPrefix + topicName

	// Get last N messages (0 to lastN-1)
	data, err := r.client.LRange(r.ctx, key, 0, int64(lastN-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get replay messages: %w", err)
	}

	messages := make([]model.Message, 0, len(data))
	for i := len(data) - 1; i >= 0; i-- { // Reverse to get oldest first
		var msg model.Message
		if err := json.Unmarshal([]byte(data[i]), &msg); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			continue
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// SaveTopicMetadata saves topic metadata to Redis
func (r *RedisStorage) SaveTopicMetadata(topicName string, subscriberCount int, messageCount int64) error {
	key := r.topicPrefix + topicName

	data := map[string]interface{}{
		"name":             topicName,
		"subscriber_count": subscriberCount,
		"message_count":    messageCount,
		"updated_at":       time.Now().Unix(),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal topic metadata: %w", err)
	}

	if err := r.client.Set(r.ctx, key, jsonData, 24*time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to save topic metadata: %w", err)
	}

	// Add to topics set
	r.client.SAdd(r.ctx, "pubsub:topics", topicName)

	return nil
}

// DeleteTopicMetadata removes topic metadata from Redis
func (r *RedisStorage) DeleteTopicMetadata(topicName string) error {
	key := r.topicPrefix + topicName
	replayKey := r.replayPrefix + topicName

	pipe := r.client.Pipeline()
	pipe.Del(r.ctx, key)
	pipe.Del(r.ctx, replayKey)
	pipe.SRem(r.ctx, "pubsub:topics", topicName)

	_, err := pipe.Exec(r.ctx)
	return err
}

// GetAllTopics retrieves all topic names from Redis
func (r *RedisStorage) GetAllTopics() ([]string, error) {
	topics, err := r.client.SMembers(r.ctx, "pubsub:topics").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get topics: %w", err)
	}
	return topics, nil
}

// GetTopicMetadata retrieves topic metadata from Redis
func (r *RedisStorage) GetTopicMetadata(topicName string) (map[string]interface{}, error) {
	key := r.topicPrefix + topicName

	data, err := r.client.Get(r.ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Topic not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get topic metadata: %w", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(data), &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal topic metadata: %w", err)
	}

	return metadata, nil
}

// IncrementMessageCount increments the message count for a topic
func (r *RedisStorage) IncrementMessageCount(topicName string) error {
	key := r.topicPrefix + topicName + ":count"
	return r.client.Incr(r.ctx, key).Err()
}

// GetMessageCount gets the total message count for a topic
func (r *RedisStorage) GetMessageCount(topicName string) (int64, error) {
	key := r.topicPrefix + topicName + ":count"
	count, err := r.client.Get(r.ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

// Ping checks if Redis is accessible
func (r *RedisStorage) Ping() error {
	return r.client.Ping(r.ctx).Err()
}

// GetStats returns Redis statistics
func (r *RedisStorage) GetStats() map[string]interface{} {
	info := r.client.Info(r.ctx, "stats").Val()
	memory := r.client.Info(r.ctx, "memory").Val()

	return map[string]interface{}{
		"connected": true,
		"stats":     info,
		"memory":    memory,
	}
}
