package slack

import (
	"fmt"
)

type HelpHandler struct {
	handlers *[]SlackSlashCommandHandler
}

func NewHelpHandler(handlers *[]SlackSlashCommandHandler) SlackSlashCommandHandler {
	return HelpHandler{
		handlers,
	}
}

func (a HelpHandler) Handle(arguments []string, request SlackSlashCommandBody) (*SlackResponse, error) {
	helpText := ""
	for i, handler := range *a.handlers {
		helpText += fmt.Sprintf("%s %s\n%s\n", handler.CommandName(), handler.CommandArguments(), handler.CommandDescription())

		if i < len(*a.handlers)-1 {
			helpText += "\n"
		}
	}

	return &SlackResponse{
		ResponseType: "ephemeral",
		Text:         helpText,
	}, nil
}

func (a HelpHandler) CommandName() string {
	return "help"
}

func (a HelpHandler) CommandArguments() string {
	return ""
}

func (a HelpHandler) CommandDescription() string {
	return "Displays a list of the available commands, their arguments, and their description"
}
