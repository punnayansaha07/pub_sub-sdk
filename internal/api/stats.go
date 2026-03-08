package api

import (
	"encoding/json"
	"net/http"
	"pubsub/internal/broker"
)

type StatsHandler struct {
	brokerInstance *broker.Broker
}

func NewStatsHandler(brokerInstance *broker.Broker) *StatsHandler {
	return &StatsHandler{
		brokerInstance: brokerInstance,
	}
}

func (handler *StatsHandler) GetStats(responseWriter http.ResponseWriter, request *http.Request) {
	stats := handler.brokerInstance.GetStatistics()

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	json.NewEncoder(responseWriter).Encode(stats)
}


func (handler *StatsHandler) GetShardStats(responseWriter http.ResponseWriter, request *http.Request) {
	shardStats := handler.brokerInstance.GetShardStatistics()

	response := map[string]interface{}{
		"shard_count": len(shardStats),
		"shards":      shardStats,
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	json.NewEncoder(responseWriter).Encode(response)
}
