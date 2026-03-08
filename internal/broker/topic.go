package broker

import (
	"errors"
	"fmt"
	"log"
	"pubsub/internal/model"
	"sync"
	"sync/atomic"
	"time"
)

type Topic struct {
	topicName              string
	subscribers            map[string]*Subscriber
	totalMessageCount      int64
	subscribersMutex       sync.RWMutex
	maxSubscribersPerTopic int
}

func NewTopic(topicName string, replayBufferSize int, maxSubscribersPerTopic int) *Topic {
	return &Topic{
		topicName:              topicName,
		subscribers:            make(map[string]*Subscriber),
		totalMessageCount:      0,
		maxSubscribersPerTopic: maxSubscribersPerTopic,
	}
}

func (topic *Topic) GetTopicName() string {
	return topic.topicName
}

func (topic *Topic) AddSubscriber(subscriber *Subscriber) error {
	topic.subscribersMutex.Lock()
	defer topic.subscribersMutex.Unlock()
	if _, exists := topic.subscribers[subscriber.GetClientID()]; exists {
		return nil
	}

	if len(topic.subscribers) >= topic.maxSubscribersPerTopic {
		return errors.New(model.ErrorCodeMaxSubscribers)
	}

	topic.subscribers[subscriber.GetClientID()] = subscriber
	subscriber.AddTopic(topic.topicName)

	log.Printf("[Topic %s] Added subscriber %s (total: %d)",
		topic.topicName, subscriber.GetClientID(), len(topic.subscribers))

	return nil
}

func (topic *Topic) RemoveSubscriber(clientID string) {
	topic.subscribersMutex.Lock()
	defer topic.subscribersMutex.Unlock()

	if subscriber, exists := topic.subscribers[clientID]; exists {
		delete(topic.subscribers, clientID)
		subscriber.RemoveTopic(topic.topicName)

		log.Printf("[Topic %s] Removed subscriber %s (remaining: %d)",
			topic.topicName, clientID, len(topic.subscribers))
	}
}

func (topic *Topic) GetSubscriberCount() int {
	topic.subscribersMutex.RLock()
	defer topic.subscribersMutex.RUnlock()
	return len(topic.subscribers)
}

func (topic *Topic) PublishMessage(message model.Message) {
	atomic.AddInt64(&topic.totalMessageCount, 1)

	serverMessage := model.ServerMessage{
		MessageType: "message",
		TopicName:   topic.topicName,
		MessageData: &message,
		Timestamp:   time.Now(),
	}

	topic.subscribersMutex.RLock()

	subscribersList := make([]*Subscriber, 0, len(topic.subscribers))
	for _, subscriber := range topic.subscribers {
		subscribersList = append(subscribersList, subscriber)
	}

	topic.subscribersMutex.RUnlock()

	slowConsumerCount := 0
	for _, subscriber := range subscribersList {
		success := subscriber.SendMessage(serverMessage)
		if !success {
			slowConsumerCount++
		}
	}

	if slowConsumerCount > 0 {
		log.Printf("[Topic %s] Published message, %d slow consumers disconnected",
			topic.topicName, slowConsumerCount)
	}
}

func (topic *Topic) GetTotalMessageCount() int64 {
	return atomic.LoadInt64(&topic.totalMessageCount)
}

func (topic *Topic) CloseAllSubscribers() {
	topic.subscribersMutex.Lock()
	defer topic.subscribersMutex.Unlock()

	infoMessage := model.ServerMessage{
		MessageType: "info",
		TopicName:   topic.topicName,
		InfoMessage: "topic_deleted",
		Timestamp:   time.Now(),
	}
	for clientID, subscriber := range topic.subscribers {
		subscriber.SendMessage(infoMessage)
		subscriber.RemoveTopic(topic.topicName)
		log.Printf("[Topic %s] Notified subscriber %s about topic deletion",
			topic.topicName, clientID)
	}
	topic.subscribers = make(map[string]*Subscriber)

	log.Printf("[Topic %s] Closed all subscribers", topic.topicName)
}

func (topic *Topic) GetSubscribersList() []string {
	topic.subscribersMutex.RLock()
	defer topic.subscribersMutex.RUnlock()

	clientIDs := make([]string, 0, len(topic.subscribers))
	for clientID := range topic.subscribers {
		clientIDs = append(clientIDs, clientID)
	}
	return clientIDs
}
func (topic *Topic) GetTopicInfo() model.TopicInfo {
	return model.TopicInfo{
		TopicName:       topic.topicName,
		SubscriberCount: topic.GetSubscriberCount(),
		MessageCount:    topic.GetTotalMessageCount(),
	}
}

func (topic *Topic) String() string {
	return fmt.Sprintf("Topic{name=%s, subscribers=%d, messages=%d}",
		topic.topicName, topic.GetSubscriberCount(), topic.GetTotalMessageCount())
}
