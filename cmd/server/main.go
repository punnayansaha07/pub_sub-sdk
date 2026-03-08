package main

import (
	"bufio"
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"pubsub/internal/api"
	"pubsub/internal/broker"
	"pubsub/internal/storage"
	"pubsub/internal/ws"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Pub/Sub Server...")

	// Initialize Redis (required)
	redisAddr := getEnvOrDefault("REDIS_ADDR", "")
	if redisAddr == "" {
		log.Fatal("❌ REDIS_ADDR environment variable is required")
	}

	redisPassword := getEnvOrDefault("REDIS_PASSWORD", "")
	log.Printf("🔌 Connecting to Redis at %s...", redisAddr)

	redisStorage, err := storage.NewRedisStorage(redisAddr, redisPassword, 0, 100)
	if err != nil {
		log.Fatalf("❌ Failed to connect to Redis: %v", err)
	}
	log.Println("✅ Redis connected successfully")

	// Initialize broker with Redis storage
	brokerInstance := broker.NewBroker(redisStorage)
	log.Println("✅ Broker initialized with Redis persistence")

	router := mux.NewRouter()

	topicsHandler := api.NewTopicsHandler(brokerInstance)
	statsHandler := api.NewStatsHandler(brokerInstance)
	healthHandler := api.NewHealthHandler(brokerInstance)
	wsHandler := ws.NewConnectionHandler(brokerInstance)

	router.HandleFunc("/topics", topicsHandler.CreateTopic).Methods("POST")
	router.HandleFunc("/topics", topicsHandler.ListTopics).Methods("GET")
	router.HandleFunc("/topics/{name}", topicsHandler.GetTopicDetails).Methods("GET")
	router.HandleFunc("/topics/{name}", topicsHandler.DeleteTopic).Methods("DELETE")

	router.HandleFunc("/stats", statsHandler.GetStats).Methods("GET")
	router.HandleFunc("/stats/shards", statsHandler.GetShardStats).Methods("GET")

	router.HandleFunc("/health", healthHandler.GetHealth).Methods("GET")
	router.HandleFunc("/ready", healthHandler.GetReadiness).Methods("GET")
	router.HandleFunc("/live", healthHandler.GetLiveness).Methods("GET")

	router.HandleFunc("/ws", wsHandler.HandleWebSocket)

	router.Use(loggingMiddleware)

	router.Use(corsMiddleware)

	serverAddress := getEnvOrDefault("SERVER_ADDRESS", ":8080")
	httpServer := &http.Server{
		Addr:         serverAddress,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("HTTP server listening on %s", serverAddress)
		log.Println("WebSocket endpoint: ws://localhost:8080/ws")
		log.Println("REST API endpoints:")
		log.Println("  POST   /topics         - Create topic")
		log.Println("  GET    /topics         - List topics")
		log.Println("  GET    /topics/{name}  - Get topic details")
		log.Println("  DELETE /topics/{name}  - Delete topic")
		log.Println("  GET    /stats          - Get statistics")
		log.Println("  GET    /health         - Get health status")
		log.Println("")
		if redisStorage != nil {
			log.Println("💾 Persistence: Redis enabled")
		} else {
			log.Println("💾 Persistence: In-memory only")
		}
		log.Println("")
		log.Println("📦 SDKs available:")
		log.Println("  JavaScript: sdk/js/")
		log.Println("  Go:         sdk/go/")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	setupGracefulShutdown(httpServer, brokerInstance)
}

func setupGracefulShutdown(httpServer *http.Server, brokerInstance *broker.Broker) {
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM)
	sig := <-shutdownSignal
	log.Printf("Received shutdown signal: %v", sig)
	log.Println("Starting graceful shutdown...")

	shutdownContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	log.Println("Shutting down HTTP server...")
	if err := httpServer.Shutdown(shutdownContext); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Server stopped gracefully")
	log.Printf("Final statistics: %s", brokerInstance.String())
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		startTime := time.Now()

		wrappedWriter := &responseWriterWrapper{
			ResponseWriter: responseWriter,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(wrappedWriter, request)
		duration := time.Since(startTime)
		log.Printf("[HTTP] %s %s - %d - %v",
			request.Method,
			request.URL.Path,
			wrappedWriter.statusCode,
			duration)
	})
}

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (wrapper *responseWriterWrapper) WriteHeader(statusCode int) {
	wrapper.statusCode = statusCode
	wrapper.ResponseWriter.WriteHeader(statusCode)
}

func (wrapper *responseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := wrapper.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Access-Control-Allow-Origin", "*")
		responseWriter.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		responseWriter.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if request.Method == "OPTIONS" {
			responseWriter.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(responseWriter, request)
	})
}

func getEnvOrDefault(envKey string, defaultValue string) string {
	value := os.Getenv(envKey)
	if value == "" {
		return defaultValue
	}
	return value
}
