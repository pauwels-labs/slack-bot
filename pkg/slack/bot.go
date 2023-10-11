package slack

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type SlackSlashCommandHandler interface {
	Handle(arguments []string, request SlackSlashCommandBody) (*SlackResponse, error)
	CommandName() string
	CommandArguments() string
	CommandDescription() string
}

type SlackBot struct {
	port       uint16
	signingKey string
	handlers   []SlackSlashCommandHandler
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
	ResponseType string `json:"response_type,omitempty"`
	Text         string `json:"text,omitempty"`
}

func NewSlackBot(port uint16, signingKey string, handlers []SlackSlashCommandHandler) SlackBot {
	helpHandler := NewHelpHandler(&handlers)
	handlers = append(handlers, helpHandler)

	return SlackBot{
		port,
		signingKey,
		handlers,
	}
}

func (sb *SlackBot) ListenAndServe(logger *zap.Logger) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", BuildHandler(logger, sb.signingKey, sb.handlers))
	return http.ListenAndServe(fmt.Sprintf(":%d", sb.port), mux)
}

func BuildHandler(logger *zap.Logger, signingKey string, handlers []SlackSlashCommandHandler) func(http.ResponseWriter, *http.Request) {
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

		// Verify that timestamp is within +/- 5 minutes from now to prevent replay attacks
		timestampHeaderInt, err := strconv.ParseInt(string(timestampHeader), 10, 64)
		if err != nil {
			logger.Error("timestamp header could not be converted to a UNIX epoch", zap.Error(err))
			return
		}
		givenTime := time.Unix(timestampHeaderInt, 0)
		timeDiffInSeconds := time.Since(givenTime).Abs().Seconds()
		if timeDiffInSeconds > 300 {
			logger.Error("timestamp header is not within five minutes of current timestamp")
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
		mac := hmac.New(sha256.New, []byte(signingKey))
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

		// Request is fully verified, acknowledge we've received it
		w.WriteHeader(http.StatusOK)

		// Place the body string back in the request so we can parse individual form fields
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		// Decode the body into a struct
		err = r.ParseForm()
		if err != nil {
			logger.Error("unable to parse form values", zap.Error(err))
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
			return
		}

		// If this is an SSL certificate verification, immediately stop execution
		if slashCommandBody.SSLCheck == "1" {
			return
		}

		// Split the command text into command and arguments
		commandTextSplit := strings.Split(slashCommandBody.Text, " ")
		command := "help"
		if len(commandTextSplit) > 0 {
			command = commandTextSplit[0]
		}
		commandArguments := []string{}
		if len(commandTextSplit) > 1 {
			commandArguments = commandTextSplit[1:]
		}

		// Identify and handle the command
		var response *SlackResponse
		for _, handler := range handlers {
			if handler.CommandName() == command {
				response, err = handler.Handle(commandArguments, slashCommandBody)
				if err != nil {
					response = &SlackResponse{
						ResponseType: "ephemeral",
						Text:         err.Error(),
					}
				}
				err = Respond(slashCommandBody.ResponseURL, response)
				if err != nil {
					logger.Error("could not send error message", zap.Error(err))
				}
				break
			}
		}
	}
}

func Respond(responseURL string, responseBody *SlackResponse) error {
	// Convert response into string
	responseString, err := json.Marshal(responseBody)
	if err != nil {
		return err
	}

	// Build response to Slack
	request, err := http.NewRequest("POST", responseURL, bytes.NewBuffer(responseString))
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
