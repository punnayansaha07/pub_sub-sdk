package broker

import (
	"errors"
	"fmt"
	"log"
	"sync"
)

type Shard struct {
	shardID int
	topicsMap map[string]*Topic
	shardMutex sync.RWMutex
	replayBufferSize int
	maxSubscribersPerTopic int
}

func NewShard(shardID int, replayBufferSize int, maxSubscribersPerTopic int) *Shard {
	return &Shard{
		shardID:                shardID,
		topicsMap:              make(map[string]*Topic),
		replayBufferSize:       replayBufferSize,
		maxSubscribersPerTopic: maxSubscribersPerTopic,
	}
}

func (shard *Shard) CreateTopic(topicName string) error {
	shard.shardMutex.Lock()
	defer shard.shardMutex.Unlock()

	if _, exists := shard.topicsMap[topicName]; exists {
		return errors.New("topic_already_exists")
	}

	newTopic := NewTopic(topicName, shard.replayBufferSize, shard.maxSubscribersPerTopic)
	shard.topicsMap[topicName] = newTopic

	log.Printf("[Shard %d] Created topic: %s", shard.shardID, topicName)

	return nil
}

func (shard *Shard) DeleteTopic(topicName string) error {
	shard.shardMutex.Lock()
	defer shard.shardMutex.Unlock()
	topic, exists := shard.topicsMap[topicName]
	if !exists {
		return errors.New("topic_not_found")
	}
	topic.CloseAllSubscribers()
	delete(shard.topicsMap, topicName)

	log.Printf("[Shard %d] Deleted topic: %s", shard.shardID, topicName)

	return nil
}

func (shard *Shard) GetTopic(topicName string) *Topic {
	shard.shardMutex.RLock()
	defer shard.shardMutex.RUnlock()

	return shard.topicsMap[topicName]
}

func (shard *Shard) TopicExists(topicName string) bool {
	shard.shardMutex.RLock()
	defer shard.shardMutex.RUnlock()

	_, exists := shard.topicsMap[topicName]
	return exists
}

func (shard *Shard) GetTopicCount() int {
	shard.shardMutex.RLock()
	defer shard.shardMutex.RUnlock()

	return len(shard.topicsMap)
}

func (shard *Shard) GetAllTopicNames() []string {
	shard.shardMutex.RLock()
	defer shard.shardMutex.RUnlock()

	topicNames := make([]string, 0, len(shard.topicsMap))
	for topicName := range shard.topicsMap {
		topicNames = append(topicNames, topicName)
	}
	return topicNames
}

func (shard *Shard) GetAllTopics() []*Topic {
	shard.shardMutex.RLock()
	defer shard.shardMutex.RUnlock()

	topics := make([]*Topic, 0, len(shard.topicsMap))
	for _, topic := range shard.topicsMap {
		topics = append(topics, topic)
	}
	return topics
}

func (shard *Shard) GetTotalSubscriberCount() int {
	shard.shardMutex.RLock()
	defer shard.shardMutex.RUnlock()

	totalSubscribers := 0
	for _, topic := range shard.topicsMap {
		totalSubscribers += topic.GetSubscriberCount()
	}
	return totalSubscribers
}

func (shard *Shard) GetShardStatistics() ShardStatistics {
	shard.shardMutex.RLock()
	defer shard.shardMutex.RUnlock()

	totalMessages := int64(0)
	totalSubscribers := 0

	for _, topic := range shard.topicsMap {
		totalMessages += topic.GetTotalMessageCount()
		totalSubscribers += topic.GetSubscriberCount()
	}

	return ShardStatistics{
		ShardID:         shard.shardID,
		TopicCount:      len(shard.topicsMap),
		SubscriberCount: totalSubscribers,
		TotalMessages:   totalMessages,
	}
}

type ShardStatistics struct {
	ShardID         int
	TopicCount      int
	SubscriberCount int
	TotalMessages   int64
}

func (shard *Shard) String() string {
	stats := shard.GetShardStatistics()
	return fmt.Sprintf("Shard{id=%d, topics=%d, subscribers=%d, messages=%d}",
		stats.ShardID, stats.TopicCount, stats.SubscriberCount, stats.TotalMessages)
}
