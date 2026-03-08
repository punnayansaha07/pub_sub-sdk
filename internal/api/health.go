package api

import (
	"encoding/json"
	"net/http"
	"pubsub/internal/broker"
)

type HealthHandler struct {
	brokerInstance *broker.Broker
}

func NewHealthHandler(brokerInstance *broker.Broker) *HealthHandler {
	return &HealthHandler{
		brokerInstance: brokerInstance,
	}
}

func (handler *HealthHandler) GetHealth(responseWriter http.ResponseWriter, request *http.Request) {
	health := handler.brokerInstance.GetHealthStatus()
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	json.NewEncoder(responseWriter).Encode(health)
}


func (handler *HealthHandler) GetReadiness(responseWriter http.ResponseWriter, request *http.Request) {
	readiness := map[string]interface{}{
		"ready":  true,
		"status": "ok",
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	json.NewEncoder(responseWriter).Encode(readiness)
}

func (handler *HealthHandler) GetLiveness(responseWriter http.ResponseWriter, request *http.Request) {
	liveness := map[string]interface{}{
		"alive":  true,
		"status": "ok",
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	json.NewEncoder(responseWriter).Encode(liveness)
}
