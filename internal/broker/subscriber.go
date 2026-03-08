package broker

import (
	"log"
	"pubsub/internal/model"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Subscriber struct {
	clientID string
	WebsocketConnection *websocket.Conn
	messageQueue chan model.ServerMessage
	isClosedFlag bool
	closedMutex sync.Mutex
	subscribedTopics map[string]bool
	topicsMutex sync.RWMutex
}

func NewSubscriber(clientID string, websocketConn *websocket.Conn, queueSize int) *Subscriber {
	return &Subscriber{
		clientID:            clientID,
		WebsocketConnection: websocketConn,
		messageQueue:        make(chan model.ServerMessage, queueSize),
		isClosedFlag:        false,
		subscribedTopics:    make(map[string]bool),
	}
}

func (subscriber *Subscriber) StartWriteLoop() {
	go subscriber.writeLoop()
}

func (subscriber *Subscriber) writeLoop() {
	defer func() {
		subscriber.WebsocketConnection.Close()
		log.Printf("[Subscriber %s] Write loop terminated", subscriber.clientID)
	}()
	const writeDeadlineSeconds = 10

	for serverMessage := range subscriber.messageQueue {
		deadline := time.Now().Add(writeDeadlineSeconds * time.Second)
		subscriber.WebsocketConnection.SetWriteDeadline(deadline)
		err := subscriber.WebsocketConnection.WriteJSON(serverMessage)
		if err != nil {
			log.Printf("[Subscriber %s] Failed to write message: %v", subscriber.clientID, err)
			subscriber.Close()
			return
		}
	}
}

func (subscriber *Subscriber) SendMessage(serverMessage model.ServerMessage) bool {
	subscriber.closedMutex.Lock()
	if subscriber.isClosedFlag {
		subscriber.closedMutex.Unlock()
		return false
	}
	subscriber.closedMutex.Unlock()

	select {
	case subscriber.messageQueue <- serverMessage:
		return true
	default:
		log.Printf("[Subscriber %s] Queue full, triggering slow consumer policy", subscriber.clientID)
		subscriber.HandleSlowConsumer()
		return false
	}
}

func (subscriber *Subscriber) HandleSlowConsumer() {
	subscriber.closedMutex.Lock()
	defer subscriber.closedMutex.Unlock()

	if subscriber.isClosedFlag {
		return
	}

	errorMessage := model.ServerMessage{
		MessageType: "error",
		Error: &model.ErrorInfo{
			ErrorCode:    model.ErrorCodeSlowConsumer,
			ErrorMessage: "Client consuming messages too slowly, disconnecting",
		},
		Timestamp: time.Now(),
	}

	subscriber.WebsocketConnection.SetWriteDeadline(time.Now().Add(1 * time.Second))
	subscriber.WebsocketConnection.WriteJSON(errorMessage)

	subscriber.WebsocketConnection.Close()

	close(subscriber.messageQueue)

	subscriber.isClosedFlag = true

	log.Printf("[Subscriber %s] Disconnected due to slow consumer policy", subscriber.clientID)
}

func (subscriber *Subscriber) Close() {
	subscriber.closedMutex.Lock()
	defer subscriber.closedMutex.Unlock()

	if subscriber.isClosedFlag {
		return
	}
	subscriber.WebsocketConnection.Close()

	close(subscriber.messageQueue)

	subscriber.isClosedFlag = true

	log.Printf("[Subscriber %s] Closed gracefully", subscriber.clientID)
}

func (subscriber *Subscriber) IsClosed() bool {
	subscriber.closedMutex.Lock()
	defer subscriber.closedMutex.Unlock()
	return subscriber.isClosedFlag
}

func (subscriber *Subscriber) GetClientID() string {
	return subscriber.clientID
}

func (subscriber *Subscriber) AddTopic(topicName string) {
	subscriber.topicsMutex.Lock()
	defer subscriber.topicsMutex.Unlock()
	subscriber.subscribedTopics[topicName] = true
}

func (subscriber *Subscriber) RemoveTopic(topicName string) {
	subscriber.topicsMutex.Lock()
	defer subscriber.topicsMutex.Unlock()
	delete(subscriber.subscribedTopics, topicName)
}

func (subscriber *Subscriber) GetSubscribedTopicCount() int {
	subscriber.topicsMutex.RLock()
	defer subscriber.topicsMutex.RUnlock()
	return len(subscriber.subscribedTopics)
}

func (subscriber *Subscriber) GetSubscribedTopics() []string {
	subscriber.topicsMutex.RLock()
	defer subscriber.topicsMutex.RUnlock()

	topics := make([]string, 0, len(subscriber.subscribedTopics))
	for topicName := range subscriber.subscribedTopics {
		topics = append(topics, topicName)
	}
	return topics
}
