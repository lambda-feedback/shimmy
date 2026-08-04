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

// TestMuEdBuildChatRequest_UserPassedThroughIntact is a regression test: the
// User field used to be typed as a flat {tone, detail, language} struct that
// didn't match the spec's nested User{type, preference{tone,detail,language},
// taskProgress} shape, silently discarding everything but the mislabeled
// fields. It must now round-trip untouched, since shimmy never inspects it.
func TestMuEdBuildChatRequest_UserPassedThroughIntact(t *testing.T) {
	user := map[string]any{
		"type": "LEARNER",
		"preference": map[string]any{
			"tone":                "FORMAL",
			"conversationalStyle": "socratic",
		},
		"taskProgress": map[string]any{
			"timeSpentOnQuestion": "30 minutes",
		},
	}
	req := runtime.MuEdChatRequest{
		Messages: []runtime.MuEdChatMessage{{Role: runtime.MuEdChatRoleUser, Content: "hi"}},
		User:     user,
	}
	body, err := runtime.MuEdBuildChatRequest(req)
	require.NoError(t, err)

	gotUser, ok := body["user"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "LEARNER", gotUser["type"])
	preference, ok := gotUser["preference"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "FORMAL", preference["tone"])
	assert.Equal(t, "socratic", preference["conversationalStyle"])
	taskProgress, ok := gotUser["taskProgress"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "30 minutes", taskProgress["timeSpentOnQuestion"])
}

// TestMuEdBuildChatRequest_ContextPassedThroughIntact is a regression test:
// the Context field used to be typed as {course, task, submission}, which
// doesn't match the spec's fully freeform "additionalProperties: true"
// context object. A real caller's context shape (e.g. {set, question,
// summary}) must survive the round trip untouched.
func TestMuEdBuildChatRequest_ContextPassedThroughIntact(t *testing.T) {
	context := map[string]any{
		"summary": "prior conversation summary",
		"set": map[string]any{
			"title":  "Fundamentals",
			"number": float64(2),
		},
		"question": map[string]any{
			"title": "Understanding Polymorphism",
		},
	}
	req := runtime.MuEdChatRequest{
		Messages: []runtime.MuEdChatMessage{{Role: runtime.MuEdChatRoleUser, Content: "hi"}},
		Context:  context,
	}
	body, err := runtime.MuEdBuildChatRequest(req)
	require.NoError(t, err)

	gotContext, ok := body["context"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "prior conversation summary", gotContext["summary"])
	set, ok := gotContext["set"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Fundamentals", set["title"])
	question, ok := gotContext["question"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Understanding Polymorphism", question["title"])
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
	output, ok := resp["output"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ASSISTANT", output["role"])
	assert.Equal(t, "Hello there!", output["content"])
	assert.NotContains(t, resp, "metadata")
}

func TestMuEdToChatResponse_MissingOutput(t *testing.T) {
	_, err := runtime.MuEdToChatResponse(map[string]any{})
	require.Error(t, err)
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
	metadata, ok := resp["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "gpt-4", metadata["model"])
}

// --- MuEdToChatHealthResponse ---

func TestMuEdToChatHealthResponse_Valid(t *testing.T) {
	result := map[string]any{
		"status": "DEGRADED",
		"capabilities": map[string]any{
			"supportsChat": true,
		},
		"statusMessage": "partially degraded",
		"version":       "1.2.3",
	}
	resp := runtime.MuEdToChatHealthResponse(result)
	assert.Equal(t, "DEGRADED", resp["status"])
	assert.Equal(t, "partially degraded", resp["statusMessage"])
	assert.Equal(t, "1.2.3", resp["version"])

	capabilities, ok := resp["capabilities"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, capabilities["supportsChat"])
	// Defaults filled in for required-but-unset spec keys.
	assert.Equal(t, "NOT_SUPPORTED", capabilities["supportsDataPolicy"])
	assert.Equal(t, []string{}, capabilities["supportedLanguages"])
	assert.Equal(t, []string{}, capabilities["supportedModels"])
	assert.Equal(t, []string{}, capabilities["supportedAPIVersions"])
}

func TestMuEdToChatHealthResponse_CapabilitiesPassedThroughIntact(t *testing.T) {
	// The worker is authoritative on its own capabilities (unlike evaluate,
	// which hardcodes them) — arbitrary worker-supplied keys must survive.
	result := map[string]any{
		"status": "OK",
		"capabilities": map[string]any{
			"supportsChat":            true,
			"supportsUserPreferences": true,
			"supportsStreaming":       false,
			"supportsDataPolicy":      "PARTIAL",
			"supportedLanguages":      []any{"en", "de"},
			"supportedModels":         []any{"gpt-4o"},
			"supportedAPIVersions":    []any{"0.1.0"},
		},
	}
	resp := runtime.MuEdToChatHealthResponse(result)
	capabilities, ok := resp["capabilities"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, capabilities["supportsChat"])
	assert.Equal(t, true, capabilities["supportsUserPreferences"])
	assert.Equal(t, false, capabilities["supportsStreaming"])
	assert.Equal(t, "PARTIAL", capabilities["supportsDataPolicy"])
	assert.Equal(t, []any{"en", "de"}, capabilities["supportedLanguages"])
	assert.Equal(t, []any{"gpt-4o"}, capabilities["supportedModels"])
	assert.Equal(t, []any{"0.1.0"}, capabilities["supportedAPIVersions"])
}

func TestMuEdToChatHealthResponse_DefaultsStatusOK(t *testing.T) {
	resp := runtime.MuEdToChatHealthResponse(map[string]any{})
	assert.Equal(t, "OK", resp["status"])
}

func TestMuEdToChatHealthResponse_DefaultsMissingCapabilities(t *testing.T) {
	resp := runtime.MuEdToChatHealthResponse(map[string]any{})
	capabilities, ok := resp["capabilities"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, false, capabilities["supportsChat"])
	assert.Equal(t, "NOT_SUPPORTED", capabilities["supportsDataPolicy"])
}

func TestMuEdToChatHealthResponse_NilSlicesDefaultToEmpty(t *testing.T) {
	resp := runtime.MuEdToChatHealthResponse(map[string]any{})

	raw, err := json.Marshal(resp)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	capabilities, ok := out["capabilities"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []any{}, capabilities["supportedLanguages"])
	assert.Equal(t, []any{}, capabilities["supportedModels"])
	assert.Equal(t, []any{}, capabilities["supportedAPIVersions"])
}
