package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	viperpit "github.com/ajpauwels/pit-of-vipers"
	"go.uber.org/zap"
)

type SlackConfig struct {
	SigningKey string `mapstructure:"signingkey"`
}

type Config struct {
	Port  uint16      `mapstructure:"port"`
	Slack SlackConfig `mapstructure:"slack"`
}

func BuildHandler(logger *zap.Logger, config *Config) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Info("handling http request", zap.String("path", r.URL.Path))

		// Ensure the request uses the POST method
		method := r.Method
		if method != "POST" {
			logger.Error("incorrect request method", zap.String("method", method))
			w.Header().Set("content-type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusMethodNotAllowed)
			io.WriteString(w, "error: must be a POST request")
			return
		}

		// Ensure the request includes a signature header
		signatureHeader := []byte(r.Header.Get("x-slack-signature"))
		if len(signatureHeader) == 0 {
			logger.Error("missing request x-slack-signature-header")
			w.Header().Set("content-type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "error: must include the x-slack-signature header")
			return
		}

		// Ensure the request includes a timestamp header
		timestampHeader := []byte(r.Header.Get("x-slack-request-timestamp"))
		if len(timestampHeader) == 0 {
			logger.Error("missing request x-slack-request-timestamp header")
			w.Header().Set("content-type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "error: must include the x-slack-request-timestamp header")
			return
		}

		// Parse the form body as a string
		body, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error("unable to parse request body", zap.Error(err))
			w.Header().Set("content-type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, "error: unable to read request body")
			return
		}

		// Create the secured request signature using the Slack signing key
		baseString := fmt.Sprintf("v0:%s:%s", timestampHeader, body)
		mac := hmac.New(sha256.New, []byte(config.Slack.SigningKey))
		bytesWritten, err := mac.Write([]byte(baseString))
		if err != nil {
			logger.Error("unable to compute request signature", zap.Error(err), zap.Int("bytesWritten", bytesWritten))
			w.Header().Set("content-type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			io.WriteString(w, "error: unable to compute request signature")
			return
		}
		signatureComputed := mac.Sum(nil)

		// Compare the generated signature with the provided signature
		if !hmac.Equal(signatureHeader, signatureComputed) {
			logger.Error("computed signature and provided signature do not match", zap.String("computed", string(signatureComputed)), zap.String("provided", string(signatureHeader)))
			w.Header().Set("content-type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "error: computed signature and provided signature do not match")
			return
		}

		w.Header().Set("content-type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "ok")
	}
}

func main() {
	// Create structured logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("couldn't initialize structured logger: %v", err)
	}
	defer logger.Sync()

	// Load configuration
	vpCh, errCh := viperpit.NewFromPathsAndName([]string{"./config"}, "base")
	for {
		select {
		case vp := <-vpCh:
			// Workaround to add ENV prefix and be able to unmarshal env-provided values
			vp.SetEnvPrefix("SERVICE")
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
