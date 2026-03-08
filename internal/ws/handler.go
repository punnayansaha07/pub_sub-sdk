package ws

import (
	"crypto/rand"
	"log"
	"math/big"
	"net/http"
	"pubsub/internal/broker"
	"pubsub/internal/model"
	"time"

	"github.com/gorilla/websocket"
	"github.com/oklog/ulid/v2"
)

var WebSocketUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type ConnectionHandler struct {
	brokerInstance *broker.Broker
}

func NewConnectionHandler(brokerInstance *broker.Broker) *ConnectionHandler {
	return &ConnectionHandler{
		brokerInstance: brokerInstance,
	}
}

func (handler *ConnectionHandler) HandleWebSocket(responseWriter http.ResponseWriter, request *http.Request) {
	websocketConn, err := WebSocketUpgrader.Upgrade(responseWriter, request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	clientID := generateClientID()

	log.Printf("[Client %s] WebSocket connection established from %s", clientID, request.RemoteAddr)

	subscriber := broker.NewSubscriber(clientID, websocketConn, broker.SubscriberQueueSize)

	subscriber.StartWriteLoop()

	handler.handleConnection(subscriber)
}

func (handler *ConnectionHandler) handleConnection(subscriber *broker.Subscriber) {
	defer func() {
		handler.cleanupSubscriber(subscriber)
		log.Printf("[Client %s] Connection closed", subscriber.GetClientID())
	}()

	websocketConn := subscriber.WebsocketConnection
	clientID := subscriber.GetClientID()

	// Set initial read deadline
	websocketConn.SetReadDeadline(time.Now().Add(5 * time.Minute))

	for {
		var clientMessage model.ClientMessage
		err := websocketConn.ReadJSON(&clientMessage)

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[Client %s] WebSocket error: %v", clientID, err)
			}
			break
		}

		// Reset read deadline after each successful message
		websocketConn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		handler.handleClientMessage(subscriber, &clientMessage)
	}
}

func (handler *ConnectionHandler) handleClientMessage(subscriber *broker.Subscriber, clientMessage *model.ClientMessage) {
	switch clientMessage.MessageType {
	case "subscribe":
		handler.handleSubscribe(subscriber, clientMessage)
	case "unsubscribe":
		handler.handleUnsubscribe(subscriber, clientMessage)
	case "publish":
		handler.handlePublish(subscriber, clientMessage)
	case "ping":
		handler.handlePing(subscriber, clientMessage)
	default:
		handler.sendError(subscriber, clientMessage.RequestID, model.ErrorCodeInvalidMessage,
			"Unknown message type: "+clientMessage.MessageType)
	}
}

func (handler *ConnectionHandler) handleSubscribe(subscriber *broker.Subscriber, clientMessage *model.ClientMessage) {
	topicName := clientMessage.TopicName
	requestID := clientMessage.RequestID
	lastNMessages := clientMessage.LastNMessages

	if topicName == "" {
		handler.sendError(subscriber, requestID, model.ErrorCodeInvalidMessage, "Topic name is required")
		return
	}

	if !handler.brokerInstance.TopicExists(topicName) {
		handler.sendError(subscriber, requestID, model.ErrorCodeTopicNotFound,
			"Topic does not exist: "+topicName)
		return
	}

	err := handler.brokerInstance.SubscribeToTopic(topicName, subscriber)
	if err != nil {
		handler.sendError(subscriber, requestID, model.ErrorCodeInternalError, err.Error())
		return
	}

	if lastNMessages > 0 {
		replayMessages, err := handler.brokerInstance.GetReplayMessages(topicName, lastNMessages)
		if err == nil {
			for _, message := range replayMessages {
				replayMessage := model.ServerMessage{
					MessageType: "message",
					TopicName:   topicName,
					MessageData: &message,
					Timestamp:   time.Now(),
				}
				subscriber.SendMessage(replayMessage)
			}
		}
	}

	handler.sendAck(subscriber, requestID, topicName, "subscribed")
}

func (handler *ConnectionHandler) handleUnsubscribe(subscriber *broker.Subscriber, clientMessage *model.ClientMessage) {
	topicName := clientMessage.TopicName
	requestID := clientMessage.RequestID

	if topicName == "" {
		handler.sendError(subscriber, requestID, model.ErrorCodeInvalidMessage, "Topic name is required")
		return
	}

	err := handler.brokerInstance.UnsubscribeFromTopic(topicName, subscriber.GetClientID())
	if err != nil {
		handler.sendError(subscriber, requestID, model.ErrorCodeInternalError, err.Error())
		return
	}

	handler.sendAck(subscriber, requestID, topicName, "unsubscribed")
}

func (handler *ConnectionHandler) handlePublish(subscriber *broker.Subscriber, clientMessage *model.ClientMessage) {
	topicName := clientMessage.TopicName
	requestID := clientMessage.RequestID

	if topicName == "" {
		handler.sendError(subscriber, requestID, model.ErrorCodeInvalidMessage, "Topic name is required")
		return
	}

	if clientMessage.MessageData == nil {
		handler.sendError(subscriber, requestID, model.ErrorCodeInvalidMessage, "Message is required")
		return
	}

	messageID := clientMessage.MessageData.MessageID
	if messageID == "" {
		messageID = generateMessageID()
	}

	message := model.Message{
		MessageID: messageID,
		Payload:   clientMessage.MessageData.Payload,
		Timestamp: time.Now(),
	}

	err := handler.brokerInstance.PublishMessage(topicName, message)
	if err != nil {
		handler.sendError(subscriber, requestID, model.ErrorCodeInternalError, err.Error())
		return
	}

	handler.sendAck(subscriber, requestID, topicName, "published")
}

func (handler *ConnectionHandler) handlePing(subscriber *broker.Subscriber, clientMessage *model.ClientMessage) {
	pongMessage := model.ServerMessage{
		MessageType: "pong",
		RequestID:   clientMessage.RequestID,
		Timestamp:   time.Now(),
	}
	subscriber.SendMessage(pongMessage)
}

func (handler *ConnectionHandler) sendAck(subscriber *broker.Subscriber, requestID string, topicName string, info string) {
	ackMessage := model.ServerMessage{
		MessageType: "ack",
		RequestID:   requestID,
		TopicName:   topicName,
		InfoMessage: info,
		Timestamp:   time.Now(),
	}
	subscriber.SendMessage(ackMessage)
}

func (handler *ConnectionHandler) sendError(subscriber *broker.Subscriber, requestID string, errorCode string, errorMessage string) {
	errorResponse := model.ServerMessage{
		MessageType: "error",
		RequestID:   requestID,
		Error: &model.ErrorInfo{
			ErrorCode:    errorCode,
			ErrorMessage: errorMessage,
		},
		Timestamp: time.Now(),
	}
	subscriber.SendMessage(errorResponse)
}

func (handler *ConnectionHandler) cleanupSubscriber(subscriber *broker.Subscriber) {
	subscribedTopics := subscriber.GetSubscribedTopics()

	for _, topicName := range subscribedTopics {
		handler.brokerInstance.UnsubscribeFromTopic(topicName, subscriber.GetClientID())
	}

	subscriber.Close()
}

func generateClientID() string {
	entropy := ulid.Monotonic(rand.Reader, 0)
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}

func generateMessageID() string {
	entropy := ulid.Monotonic(rand.Reader, 0)
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[num.Int64()]
	}
	return string(result)
}
