package slack

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

	_ = NewSlackBot(8080, "abc", []SlackSlashCommandHandler{})
}
