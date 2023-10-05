package main

import (
	"go.uber.org/zap"
	"testing"
)

func TestHandlerWithLogger(t *testing.T) {
	// Create structured logger
	logger, err := zap.NewProduction()
	if err != nil {
		t.Errorf("couldn't initialize structured logger: %v", err)
	}
	defer logger.Sync()

	// Create config
	config := Config{
		Port: 8080,
		Slack: SlackConfig{
			SigningKey: "abc",
		},
	}

	_ = BuildHandler(logger, &config)
}
