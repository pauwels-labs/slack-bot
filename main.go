package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	viperpit "github.com/ajpauwels/pit-of-vipers"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type SlackConfig struct {
	SigningKey string `mapstructure:"signingkey"`
}

type Config struct {
	Port  uint16      `mapstructure:"port"`
	Slack SlackConfig `mapstructure:"slack"`
}

type SlackSlashCommandBody struct {
	Command     string `mapstructure:"command,omitempty"`
	Text        string `mapstructure:"text,omitempty"`
	ResponseURL string `mapstructure:"response_url,omitempty"`
	TriggerID   string `mapstructure:"trigger_id,omitempty"`
	UserID      string `mapstructure:"user_id,omitempty"`
	APIAppID    string `mapstructure:"api_add_id,omitempty"`
	SSLCheck    string `mapstructure:"ssl_check,omitempty"`
}

type SlackResponse struct {
	ResponseType string `mapstructure:"response_type" json:"response_type,omitempty"`
	Text         string `mapstructure:"text" json:"text,omitempty"`
}

func RespondWithError(requestURL string, errText string) error {
	// Create request
	requestBody := SlackResponse{
		ResponseType: "ephemeral",
		Text:         errText,
	}
	requestString, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}
	request, err := http.NewRequest("POST", requestURL, bytes.NewBuffer(requestString))
	if err != nil {
		return err
	}
	request.Header.Set("content-type", "application/json; charset=utf-8")

	// Execute request
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return nil
}

func RespondWithHelp(requestURL string, logger *zap.Logger) error {
	helpText := "This command currently has no features :("

	// Create request
	requestBody := SlackResponse{
		ResponseType: "ephemeral",
		Text:         helpText,
	}
	requestString, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}
	request, err := http.NewRequest("POST", requestURL, bytes.NewBuffer(requestString))
	if err != nil {
		return err
	}
	request.Header.Set("content-type", "application/json; charset=utf-8")

	logger.Info("sending help response", zap.String("requestURL", requestURL), zap.String("requestString", string(requestString)))

	// Execute request
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return nil
}

func BuildHandler(logger *zap.Logger, config *Config) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Ensure the request uses the POST method
		method := r.Method
		if method != "POST" {
			logger.Error("incorrect request method", zap.String("method", method))
			return
		}

		// Ensure the request uses the application/x-www-form-urlencoded content-type
		contentType := r.Header.Get("content-type")
		if contentType != "application/x-www-form-urlencoded" {
			logger.Error("incorrect content-type", zap.String("contentType", contentType))
			return
		}

		// Ensure the request includes a signature header
		signatureHeader := r.Header.Get("x-slack-signature")
		if len(signatureHeader) == 0 {
			logger.Error("missing request x-slack-signature-header")
			return
		}

		// Ensure the request includes a timestamp header
		timestampHeader := []byte(r.Header.Get("x-slack-request-timestamp"))
		if len(timestampHeader) == 0 {
			logger.Error("missing request x-slack-request-timestamp header")
			return
		}

		// Generate a string of the request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error("unable to parse request body", zap.Error(err))
			return
		}

		// Create the secured request signature using the Slack signing key
		baseString := fmt.Sprintf("v0:%s:%s", timestampHeader, body)
		mac := hmac.New(sha256.New, []byte(config.Slack.SigningKey))
		bytesWritten, err := mac.Write([]byte(baseString))
		if err != nil {
			logger.Error("unable to compute request signature", zap.Error(err), zap.Int("bytesWritten", bytesWritten))
			return
		}
		signatureComputed := mac.Sum(nil)
		signatureComputedHex := hex.EncodeToString(signatureComputed)
		signatureComputedFormatted := fmt.Sprintf("v0=%s", signatureComputedHex)

		// Compare the generated signature with the provided signature
		if signatureComputedFormatted != signatureHeader {
			logger.Error("computed signature and provided signature do not match", zap.String("computed", signatureComputedFormatted), zap.String("provided", signatureHeader))
			return
		}

		// Place the body string back in the request so we can parse individual form fields
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		// Decode the body into a struct
		err = r.ParseForm()
		if err != nil {
			logger.Error("unable to parse form values", zap.Error(err))
			w.WriteHeader(http.StatusOK)
			return
		}
		undecodedForm := map[string]string{}
		for key, element := range r.Form {
			undecodedForm[key] = element[0]
		}
		var slashCommandBody SlackSlashCommandBody
		err = mapstructure.Decode(undecodedForm, &slashCommandBody)
		if err != nil {
			logger.Error("unable to decode form values into struct", zap.Error(err))
			w.WriteHeader(http.StatusOK)
			return
		}

		// If this is an SSL certificate verification, immediately return 200
		if slashCommandBody.SSLCheck == "1" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Post help as an ephemeral message
		if slashCommandBody.Text[0:4] == "help" {
			w.WriteHeader(http.StatusOK)
			err = RespondWithHelp(slashCommandBody.ResponseURL, logger)
			if err != nil {
				logger.Error("could not respond to help command", zap.Error(err))
				err = RespondWithError(slashCommandBody.ResponseURL, "there was an error sending the help text")
				if err != nil {
					logger.Error("could not send error message", zap.Error(err))
				}
			}
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

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
			var config Config
			vp.Unmarshal(&config)

			logger.Info("config", zap.Uint16("port", config.Port), zap.String("slack.signingkey", string(config.Slack.SigningKey)))

			// Define handlers
			mux := http.NewServeMux()
			mux.HandleFunc("/", BuildHandler(logger, &config))

			// Start server
			logger.Info("starting server", zap.Uint16("port", config.Port))
			err = http.ListenAndServe(fmt.Sprintf(":%d", config.Port), mux)
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
