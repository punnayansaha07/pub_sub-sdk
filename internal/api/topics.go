package api

import (
	"encoding/json"
	"log"
	"net/http"
	"pubsub/internal/broker"
	"pubsub/internal/model"

	"github.com/gorilla/mux"
)

type TopicsHandler struct {
	brokerInstance *broker.Broker
}

func NewTopicsHandler(brokerInstance *broker.Broker) *TopicsHandler {
	return &TopicsHandler{
		brokerInstance: brokerInstance,
	}
}

func (handler *TopicsHandler) CreateTopic(responseWriter http.ResponseWriter, request *http.Request) {
	var createRequest model.CreateTopicRequest
	err := json.NewDecoder(request.Body).Decode(&createRequest)
	if err != nil {
		handler.sendErrorResponse(responseWriter, http.StatusBadRequest, "Invalid request body")
		return
	}

	if createRequest.TopicName == "" {
		handler.sendErrorResponse(responseWriter, http.StatusBadRequest, "Topic name is required")
		return
	}

	err = handler.brokerInstance.CreateTopic(createRequest.TopicName)
	if err != nil {
		if err.Error() == "topic_already_exists" {
			handler.sendErrorResponse(responseWriter, http.StatusConflict, "Topic already exists")
			return
		}
		if err.Error() == model.ErrorCodeMaxTopicsReached {
			handler.sendErrorResponse(responseWriter, http.StatusServiceUnavailable, "Maximum topics limit reached")
			return
		}
		handler.sendErrorResponse(responseWriter, http.StatusInternalServerError, err.Error())
		return
	}

	response := model.CreateTopicResponse{
		Success:   true,
		TopicName: createRequest.TopicName,
	}

	handler.sendJSONResponse(responseWriter, http.StatusCreated, response)
	log.Printf("[API] Topic created: %s", createRequest.TopicName)
}

func (handler *TopicsHandler) DeleteTopic(responseWriter http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	topicName := vars["name"]

	if topicName == "" {
		handler.sendErrorResponse(responseWriter, http.StatusBadRequest, "Topic name is required")
		return
	}

	err := handler.brokerInstance.DeleteTopic(topicName)
	if err != nil {
		if err.Error() == "topic_not_found" {
			handler.sendErrorResponse(responseWriter, http.StatusNotFound, "Topic not found")
			return
		}
		handler.sendErrorResponse(responseWriter, http.StatusInternalServerError, err.Error())
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Topic deleted successfully",
		"topic":   topicName,
	}

	handler.sendJSONResponse(responseWriter, http.StatusOK, response)
	log.Printf("[API] Topic deleted: %s", topicName)
}

func (handler *TopicsHandler) ListTopics(responseWriter http.ResponseWriter, request *http.Request) {
	topics := handler.brokerInstance.ListAllTopics()

	response := model.ListTopicsResponse{
		Topics: topics,
	}

	handler.sendJSONResponse(responseWriter, http.StatusOK, response)
}

func (handler *TopicsHandler) GetTopicDetails(responseWriter http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	topicName := vars["name"]

	if topicName == "" {
		handler.sendErrorResponse(responseWriter, http.StatusBadRequest, "Topic name is required")
		return
	}

	topic, err := handler.brokerInstance.GetTopic(topicName)
	if err != nil {
		handler.sendErrorResponse(responseWriter, http.StatusNotFound, "Topic not found")
		return
	}

	response := topic.GetTopicInfo()

	handler.sendJSONResponse(responseWriter, http.StatusOK, response)
}

func (handler *TopicsHandler) sendJSONResponse(responseWriter http.ResponseWriter, statusCode int, data interface{}) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	json.NewEncoder(responseWriter).Encode(data)
}

func (handler *TopicsHandler) sendErrorResponse(responseWriter http.ResponseWriter, statusCode int, message string) {
	errorResponse := map[string]interface{}{
		"success": false,
		"error":   message,
	}
	handler.sendJSONResponse(responseWriter, statusCode, errorResponse)
}
