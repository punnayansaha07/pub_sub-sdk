package model

import "time"

type Message struct {
	MessageID string      `json:"id"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"ts"`
}

type ErrorInfo struct {
	ErrorCode    string `json:"code"`
	ErrorMessage string `json:"message"`
}

type TopicInfo struct {
	TopicName       string `json:"name"`
	SubscriberCount int    `json:"subscribers"`
	MessageCount    int64  `json:"message_count,omitempty"`
}

type Stats struct {
	TotalTopics      int         `json:"total_topics"`
	TotalSubscribers int         `json:"total_subscribers"`
	TopicStats       []TopicInfo `json:"topics"`
}

type Health struct {
	Status           string `json:"status"`
	UptimeSeconds    int64  `json:"uptime_sec"`
	TotalTopics      int    `json:"topics"`
	TotalSubscribers int    `json:"subscribers"`
}
