package runtime

import (
	"encoding/json"
	"fmt"
)

// ChatRequest is the dispatcher-level request for the chat command.
type ChatRequest struct {
	Data map[string]any
}

// ChatResponse is the dispatcher-level response for the chat command.
type ChatResponse struct {
	Data map[string]any
}

type MuEdChatRole string

const (
	MuEdChatRoleUser      MuEdChatRole = "USER"
	MuEdChatRoleAssistant MuEdChatRole = "ASSISTANT"
	MuEdChatRoleSystem    MuEdChatRole = "SYSTEM"
	MuEdChatRoleTool      MuEdChatRole = "TOOL"
)

type MuEdChatMessage struct {
	Role    MuEdChatRole `json:"role"`
	Content string       `json:"content"`
}

// MuEdChatRequest is the request body for the chat endpoint. Only messages
// and conversationId have a fixed shape per the µEd spec — user, context,
// and configuration are all declared additionalProperties/freeform (or, for
// user, nested under a User schema that isn't worth flattening here), and
// are never inspected by shimmy itself; they only flow straight through to
// the worker. Typing them narrowly risks silently dropping fields that don't
// match a hand-picked sub-schema, so they stay as map[string]any, matching
// the convention used for task-specific data in evaluate.go (e.g.
// MuEdSubmission.Content, MuEdTask.ReferenceSolution).
type MuEdChatRequest struct {
	Messages       []MuEdChatMessage `json:"messages"`
	ConversationID string            `json:"conversationId,omitempty"`
	User           map[string]any    `json:"user,omitempty"`
	Context        map[string]any    `json:"context,omitempty"`
	Configuration  map[string]any    `json:"configuration,omitempty"`
}

type MuEdChatHealthStatus string

const (
	MuEdChatHealthStatusOK          MuEdChatHealthStatus = "OK"
	MuEdChatHealthStatusDegraded    MuEdChatHealthStatus = "DEGRADED"
	MuEdChatHealthStatusUnavailable MuEdChatHealthStatus = "UNAVAILABLE"
)

// MuEdBuildChatRequest converts a MuEdChatRequest to the map sent to the worker.
func MuEdBuildChatRequest(req MuEdChatRequest) (map[string]any, error) {
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("messages must not be empty")
	}
	b, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat request: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("failed to build chat request: %w", err)
	}
	return m, nil
}

// MuEdToChatResponse transforms a worker result map into a µEd chat response map.
func MuEdToChatResponse(result map[string]any) (map[string]any, error) {
	output, ok := result["output"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("chat response missing output")
	}

	role, _ := output["role"].(string)
	if role == "" {
		return nil, fmt.Errorf("chat response missing output role")
	}

	content, _ := output["content"].(string)
	if content == "" {
		return nil, fmt.Errorf("chat response missing output content")
	}

	resp := map[string]any{
		"output": map[string]any{
			"role":    role,
			"content": content,
		},
	}
	if metadata, ok := result["metadata"].(map[string]any); ok {
		resp["metadata"] = metadata
	}
	return resp, nil
}

// MuEdToChatHealthResponse transforms a worker result map into a µEd chat
// health response map. Unlike evaluate's health capabilities (which shimmy
// hardcodes itself), a chat worker is authoritative on what it supports, so
// this passes the worker's capabilities through largely as-is — it only
// fills in the spec's required keys/defaults and normalises nil slices to
// empty ones so they serialise as [] not null.
func MuEdToChatHealthResponse(result map[string]any) map[string]any {
	status, _ := result["status"].(string)
	if status == "" {
		status = string(MuEdChatHealthStatusOK)
	}

	capabilities, ok := result["capabilities"].(map[string]any)
	if !ok {
		capabilities = map[string]any{}
	}
	if _, ok := capabilities["supportsChat"]; !ok {
		capabilities["supportsChat"] = false
	}
	if _, ok := capabilities["supportsDataPolicy"]; !ok {
		capabilities["supportsDataPolicy"] = "NOT_SUPPORTED"
	}
	for _, key := range []string{"supportedLanguages", "supportedModels", "supportedAPIVersions"} {
		if capabilities[key] == nil {
			capabilities[key] = []string{}
		}
	}

	resp := map[string]any{
		"status":       status,
		"capabilities": capabilities,
	}
	if msg, ok := result["statusMessage"].(string); ok {
		resp["statusMessage"] = msg
	}
	if version, ok := result["version"].(string); ok {
		resp["version"] = version
	}
	return resp
}
