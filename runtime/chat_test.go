package runtime_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lambda-feedback/shimmy/runtime"
)

// --- MuEdBuildChatRequest ---

func TestMuEdBuildChatRequest_Valid(t *testing.T) {
	req := runtime.MuEdChatRequest{
		Messages: []runtime.MuEdChatMessage{
			{Role: runtime.MuEdChatRoleUser, Content: "hello"},
		},
	}
	body, err := runtime.MuEdBuildChatRequest(req)
	require.NoError(t, err)

	msgs, ok := body["messages"].([]any)
	require.True(t, ok)
	require.Len(t, msgs, 1)
	msg := msgs[0].(map[string]any)
	assert.Equal(t, "USER", msg["role"])
	assert.Equal(t, "hello", msg["content"])
}

func TestMuEdBuildChatRequest_EmptyMessages(t *testing.T) {
	req := runtime.MuEdChatRequest{Messages: []runtime.MuEdChatMessage{}}
	_, err := runtime.MuEdBuildChatRequest(req)
	require.Error(t, err)
}

func TestMuEdBuildChatRequest_NilMessages(t *testing.T) {
	req := runtime.MuEdChatRequest{}
	_, err := runtime.MuEdBuildChatRequest(req)
	require.Error(t, err)
}

func TestMuEdBuildChatRequest_OptionalFieldsOmitted(t *testing.T) {
	req := runtime.MuEdChatRequest{
		Messages: []runtime.MuEdChatMessage{
			{Role: runtime.MuEdChatRoleUser, Content: "hi"},
		},
	}
	body, err := runtime.MuEdBuildChatRequest(req)
	require.NoError(t, err)

	_, hasUser := body["user"]
	_, hasContext := body["context"]
	_, hasConversationID := body["conversationId"]
	assert.False(t, hasUser)
	assert.False(t, hasContext)
	assert.False(t, hasConversationID)
}

func TestMuEdBuildChatRequest_ConversationIDIncluded(t *testing.T) {
	req := runtime.MuEdChatRequest{
		Messages:       []runtime.MuEdChatMessage{{Role: runtime.MuEdChatRoleUser, Content: "hi"}},
		ConversationID: "abc-123",
	}
	body, err := runtime.MuEdBuildChatRequest(req)
	require.NoError(t, err)
	assert.Equal(t, "abc-123", body["conversationId"])
}

// --- MuEdToChatResponse ---

func TestMuEdToChatResponse_Valid(t *testing.T) {
	result := map[string]any{
		"output": map[string]any{
			"role":    "ASSISTANT",
			"content": "Hello there!",
		},
	}
	resp, err := runtime.MuEdToChatResponse(result)
	require.NoError(t, err)
	assert.Equal(t, runtime.MuEdChatRoleAssistant, resp.Output.Role)
	assert.Equal(t, "Hello there!", resp.Output.Content)
	assert.Nil(t, resp.Metadata)
}

func TestMuEdToChatResponse_MissingRole(t *testing.T) {
	result := map[string]any{
		"output": map[string]any{
			"content": "Hello",
		},
	}
	_, err := runtime.MuEdToChatResponse(result)
	require.Error(t, err)
}

func TestMuEdToChatResponse_MissingContent(t *testing.T) {
	result := map[string]any{
		"output": map[string]any{
			"role": "ASSISTANT",
		},
	}
	_, err := runtime.MuEdToChatResponse(result)
	require.Error(t, err)
}

func TestMuEdToChatResponse_MetadataForwarded(t *testing.T) {
	result := map[string]any{
		"output": map[string]any{
			"role":    "ASSISTANT",
			"content": "Hi",
		},
		"metadata": map[string]any{
			"model": "gpt-4",
		},
	}
	resp, err := runtime.MuEdToChatResponse(result)
	require.NoError(t, err)
	require.NotNil(t, resp.Metadata)
	assert.Equal(t, "gpt-4", resp.Metadata.Model)
}

// --- MuEdToChatHealthResponse ---

func TestMuEdToChatHealthResponse_Valid(t *testing.T) {
	result := map[string]any{
		"status": "DEGRADED",
		"capabilities": map[string]any{
			"chat": true,
		},
		"supportedLanguages":   []any{"en"},
		"supportedModels":      []any{"gpt-4"},
		"supportedAPIVersions": []any{"1.0"},
	}
	resp := runtime.MuEdToChatHealthResponse(result)
	assert.Equal(t, runtime.MuEdChatHealthStatusDegraded, resp.Status)
	assert.True(t, resp.Capabilities.Chat)
	assert.Equal(t, []string{"en"}, resp.SupportedLanguages)
	assert.Equal(t, []string{"gpt-4"}, resp.SupportedModels)
	assert.Equal(t, []string{"1.0"}, resp.SupportedAPIVersions)
}

func TestMuEdToChatHealthResponse_DefaultsStatusOK(t *testing.T) {
	resp := runtime.MuEdToChatHealthResponse(map[string]any{})
	assert.Equal(t, runtime.MuEdChatHealthStatusOK, resp.Status)
}

func TestMuEdToChatHealthResponse_NilSlicesDefaultToEmpty(t *testing.T) {
	resp := runtime.MuEdToChatHealthResponse(map[string]any{})

	raw, err := json.Marshal(resp)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	assert.Equal(t, []any{}, out["supportedLanguages"])
	assert.Equal(t, []any{}, out["supportedModels"])
	assert.Equal(t, []any{}, out["supportedAPIVersions"])
}
