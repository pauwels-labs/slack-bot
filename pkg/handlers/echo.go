package handlers

import (
	"github.com/pauwels-labs/slack-bot/pkg/slack"
	"strings"
)

type EchoHandler struct {
}

func NewEchoHandler() slack.SlackSlashCommandHandler {
	return EchoHandler{}
}

func (a EchoHandler) Handle(arguments []string, request slack.SlackSlashCommandBody) (*slack.SlackResponse, error) {
	return &slack.SlackResponse{
		ResponseType: "in_channel",
		Text:         strings.Join(arguments, " "),
	}, nil
}

func (a EchoHandler) CommandName() string {
	return "echo"
}

func (a EchoHandler) CommandArguments() string {
	return "[words...]"
}

func (a EchoHandler) CommandDescription() string {
	return "Accepts any number of arguments and echoes them back to the channel"
}
