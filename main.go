package main

import (
	"log"
	"net/http"

	"go.uber.org/zap"
)

func HandlerWithLogger(logger *zap.Logger) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Info("handling http request", zap.String("path", r.URL.Path))
	}
}

func main() {
	// Create structured logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("couldn't initialize structured logger: %v", err)
	}
	defer logger.Sync()

	// Define handlers
	http.HandleFunc("/", HandlerWithLogger(logger))

	// Start server
	logger.Info("starting server", zap.Int("port", 8080))
	err = http.ListenAndServe(":8080", nil)
	logger.Error("failed to start http server", zap.Error(err))
}
