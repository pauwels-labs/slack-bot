package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	viperpit "github.com/ajpauwels/pit-of-vipers"
	"github.com/pauwels-labs/slack-bot/internal/config"
	"github.com/pauwels-labs/slack-bot/pkg/handlers"
	"github.com/pauwels-labs/slack-bot/pkg/slack"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func main() {
	// Create structured logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("couldn't initialize structured logger: %v", err)
	}
	defer logger.Sync()

	// Load env-specific configuration
	env := os.Getenv("APPCFG_meta_env")
	configPath := "./config"
	if len(env) <= 0 {
		env = "local"
	}
	if env != "local" {
		configPath = "/etc/slack-bot/config"
	}

	// Create viper instances for base and env-specific config files
	baseViper := viper.New()
	baseViper.AddConfigPath(configPath)
	baseViper.SetConfigName("base")
	envViper := viper.New()
	envViper.AddConfigPath(configPath)
	envViper.SetConfigName(env)

	vpCh, errCh := viperpit.New([]*viper.Viper{baseViper, envViper})
	for {
		select {
		case vp := <-vpCh:
			// Workaround to add ENV prefix and be able to unmarshal env-provided values
			vp.SetEnvPrefix("APPCFG")
			vp.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
			for _, key := range vp.AllKeys() {
				val := vp.Get(key)
				vp.Set(key, val)
			}

			// Unmarshal config into struct
			var config config.Config
			vp.Unmarshal(&config)

			logger.Info("config", zap.Uint16("port", config.Port), zap.String("slack.signingkey", string(config.Slack.SigningKey)))

			// Create slack bot server
			slackBot := slack.NewSlackBot(config.Port, config.Slack.SigningKey, CreateHandlers())
			logger.Info("starting server", zap.Uint16("port", config.Port))
			err := slackBot.ListenAndServe(logger)

			// Handle normal shutdown and server start errors
			if errors.Is(err, http.ErrServerClosed) {
				logger.Info("server has shutdown normally")
				break
			} else {
				logger.Fatal("failed to start http server", zap.Error(err))
			}
		case err := <-errCh:
			logger.Error("error loading config", zap.Error(err))
		}
	}
}

func CreateHandlers() []slack.SlackSlashCommandHandler {
	echoHandler := handlers.NewEchoHandler()
	return []slack.SlackSlashCommandHandler{echoHandler}
}
