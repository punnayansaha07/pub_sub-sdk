package broker

import (
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"pubsub/internal/model"
	"pubsub/internal/storage"
	"time"
)

const (
	ShardCount             = 32
	SubscriberQueueSize    = 100
	ReplayBufferSize       = 100
	MaxTopicsLimit         = 10000
	MaxSubscribersPerTopic = 50000
)

type Broker struct {
	shards    []*Shard
	startTime time.Time
	storage   *storage.RedisStorage // Redis storage (required)
}

// NewBroker creates a new broker with Redis storage (required)
func NewBroker(redisStorage *storage.RedisStorage) *Broker {
	if redisStorage == nil {
		log.Fatal("Redis storage is required")
	}

	log.Printf("Initializing broker with %d shards", ShardCount)
	shards := make([]*Shard, ShardCount)
	for i := 0; i < ShardCount; i++ {
		shards[i] = NewShard(i, ReplayBufferSize, MaxSubscribersPerTopic)
	}

	broker := &Broker{
		shards:    shards,
		startTime: time.Now(),
		storage:   redisStorage,
	}

	log.Printf("Broker initialized with Redis persistence")
	return broker
}

func (broker *Broker) selectShardForTopic(topicName string) *Shard {
	hasher := fnv.New32a()
	hasher.Write([]byte(topicName))
	hashValue := hasher.Sum32()
	shardIndex := int(hashValue % uint32(ShardCount))
	return broker.shards[shardIndex]
}

func (broker *Broker) CreateTopic(topicName string) error {
	if topicName == "" {
		return errors.New("topic_name_empty")
	}
	currentTopicCount := broker.GetTotalTopicCount()
	if currentTopicCount >= MaxTopicsLimit {
		log.Printf("Cannot create topic %s: max topics limit reached (%d)", topicName, MaxTopicsLimit)
		return errors.New(model.ErrorCodeMaxTopicsReached)
	}

	shard := broker.selectShardForTopic(topicName)
	err := shard.CreateTopic(topicName)

	if err != nil {
		return err
	}

	log.Printf("Topic created: %s (shard %d)", topicName, shard.shardID)
	return nil
}

func (broker *Broker) DeleteTopic(topicName string) error {
	shard := broker.selectShardForTopic(topicName)
	err := shard.DeleteTopic(topicName)

	if err != nil {
		return err
	}

	log.Printf("Topic deleted: %s", topicName)
	return nil
}

func (broker *Broker) GetTopic(topicName string) (*Topic, error) {
	shard := broker.selectShardForTopic(topicName)
	topic := shard.GetTopic(topicName)

	if topic == nil {
		return nil, errors.New(model.ErrorCodeTopicNotFound)
	}

	return topic, nil
}

func (broker *Broker) TopicExists(topicName string) bool {
	shard := broker.selectShardForTopic(topicName)
	return shard.TopicExists(topicName)
}

func (broker *Broker) PublishMessage(topicName string, message model.Message) error {
	topic, err := broker.GetTopic(topicName)
	if err != nil {
		return err
	}

	// Save to Redis storage (required)
	if err := broker.storage.SaveMessage(topicName, message); err != nil {
		log.Printf("Failed to save message to Redis: %v", err)
		return fmt.Errorf("failed to persist message: %w", err)
	}

	topic.PublishMessage(message)

	return nil
}

func (broker *Broker) SubscribeToTopic(topicName string, subscriber *Subscriber) error {
	topic, err := broker.GetTopic(topicName)
	if err != nil {
		return err
	}

	err = topic.AddSubscriber(subscriber)
	if err != nil {
		return err
	}

	log.Printf("Subscriber %s subscribed to topic %s", subscriber.GetClientID(), topicName)
	return nil
}

func (broker *Broker) UnsubscribeFromTopic(topicName string, clientID string) error {
	topic, err := broker.GetTopic(topicName)
	if err != nil {
		return err
	}

	topic.RemoveSubscriber(clientID)

	log.Printf("Subscriber %s unsubscribed from topic %s", clientID, topicName)
	return nil
}

func (broker *Broker) GetReplayMessages(topicName string, lastNMessages int) ([]model.Message, error) {
	// Get messages from Redis storage (required)
	messages, err := broker.storage.GetReplayMessages(topicName, lastNMessages)
	if err != nil {
		log.Printf("Failed to get replay messages from Redis: %v", err)
		return nil, fmt.Errorf("failed to get replay messages: %w", err)
	}

	return messages, nil
}

func (broker *Broker) ListAllTopics() []model.TopicInfo {
	allTopics := make([]model.TopicInfo, 0)

	for _, shard := range broker.shards {
		shardTopics := shard.GetAllTopics()
		for _, topic := range shardTopics {
			allTopics = append(allTopics, topic.GetTopicInfo())
		}
	}

	return allTopics
}

func (broker *Broker) GetStatistics() model.Stats {
	topicInfoList := broker.ListAllTopics()

	totalSubscribers := 0
	for _, topicInfo := range topicInfoList {
		totalSubscribers += topicInfo.SubscriberCount
	}

	return model.Stats{
		TotalTopics:      len(topicInfoList),
		TotalSubscribers: totalSubscribers,
		TopicStats:       topicInfoList,
	}
}

func (broker *Broker) GetHealthStatus() model.Health {
	stats := broker.GetStatistics()
	uptimeSeconds := int64(time.Since(broker.startTime).Seconds())

	return model.Health{
		Status:           "ok",
		UptimeSeconds:    uptimeSeconds,
		TotalTopics:      stats.TotalTopics,
		TotalSubscribers: stats.TotalSubscribers,
	}
}

func (broker *Broker) GetTotalTopicCount() int {
	totalTopics := 0
	for _, shard := range broker.shards {
		totalTopics += shard.GetTopicCount()
	}
	return totalTopics
}

func (broker *Broker) GetTotalSubscriberCount() int {
	totalSubscribers := 0
	for _, shard := range broker.shards {
		totalSubscribers += shard.GetTotalSubscriberCount()
	}
	return totalSubscribers
}

func (broker *Broker) GetShardStatistics() []ShardStatistics {
	shardStats := make([]ShardStatistics, ShardCount)
	for i, shard := range broker.shards {
		shardStats[i] = shard.GetShardStatistics()
	}
	return shardStats
}

func (broker *Broker) String() string {
	return fmt.Sprintf("Broker{topics=%d, subscribers=%d, uptime=%s}",
		broker.GetTotalTopicCount(),
		broker.GetTotalSubscriberCount(),
		time.Since(broker.startTime).String())
}
