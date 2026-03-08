package model

import "time"

type ClientMessage struct {
	MessageType   string   `json:"type"`
	TopicName     string   `json:"topic,omitempty"`
	ClientID      string   `json:"client_id,omitempty"`
	LastNMessages int      `json:"last_n,omitempty"`
	RequestID     string   `json:"request_id,omitempty"`
	MessageData   *Message `json:"message,omitempty"`
}

type ServerMessage struct {
	MessageType string     `json:"type"`
	TopicName   string     `json:"topic,omitempty"`
	RequestID   string     `json:"request_id,omitempty"`
	MessageData *Message   `json:"message,omitempty"`
	Error       *ErrorInfo `json:"error,omitempty"`
	Timestamp   time.Time  `json:"ts"`
	InfoMessage string     `json:"info,omitempty"`
}

type CreateTopicRequest struct {
	TopicName string `json:"name"`
}

type CreateTopicResponse struct {
	Success      bool   `json:"success"`
	TopicName    string `json:"name,omitempty"`
	ErrorMessage string `json:"error,omitempty"`
}

type ListTopicsResponse struct {
	Topics []TopicInfo `json:"topics"`
}

// Error code constants
const (
	ErrorCodeTopicNotFound      = "TOPIC_NOT_FOUND"
	ErrorCodeTopicAlreadyExists = "TOPIC_ALREADY_EXISTS"
	ErrorCodeSlowConsumer       = "SLOW_CONSUMER"
	ErrorCodeInvalidMessage     = "INVALID_MESSAGE"
	ErrorCodeMaxTopicsReached   = "MAX_TOPICS_REACHED"
	ErrorCodeMaxSubscribers     = "MAX_SUBSCRIBERS_REACHED"
	ErrorCodeInternalError      = "INTERNAL_ERROR"
)
