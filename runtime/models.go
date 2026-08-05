package runtime

import (
	"strings"
)

// Command is a command that can be sent between the runtime and the handler.
type Command string

const (
	// CommandPreview is the command to get a preview for the response.
	CommandPreview Command = "preview"

	// CommandEvaluate is the command to evaluate the response.
	CommandEvaluate Command = "eval"

	// CommandEvaluateHealth is the command for healthcheck
	CommandEvaluateHealth = "healthcheck"

	// CommandChat is the command for chat.
	CommandChat Command = "chat"

	// CommandChatHealth is the command for the chat health check.
	CommandChatHealth Command = "chat/health"
)

// ParseCommand parses a command from a given path.
func ParseCommand(path string) (Command, bool) {
	switch strings.ToLower(path) {
	case "eval":
		return CommandEvaluate, true
	case "preview":
		return CommandPreview, true
	case "healthcheck":
		return CommandEvaluateHealth, true
	case "chat":
		return CommandChat, true
	case "chat/health":
		return CommandChatHealth, true
	}

	return "", false
}

type EvaluationRequest struct {
	Command Command

	Data map[string]any
}

// NewRequestMessage creates a new message.
func NewRequestMessage(command Command, data map[string]any) EvaluationRequest {
	return EvaluationRequest{
		Command: command,
		Data:    data,
	}
}

type EvaluationResponse map[string]any
