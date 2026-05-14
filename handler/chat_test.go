package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lambda-feedback/shimmy/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Helpers ---

func chatRequestBody(t *testing.T) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"messages": []map[string]any{
			{"role": "USER", "content": "hello"},
		},
	})
	require.NoError(t, err)
	return b
}

func chatRuntimeResponse(role, content string) runtime.EvaluationResponse {
	return runtime.EvaluationResponse{
		"command": "chat",
		"result": map[string]any{
			"output": map[string]any{
				"role":    role,
				"content": content,
			},
		},
	}
}

// --- ServeChat tests ---

func TestServeChat_Success(t *testing.T) {
	mockRuntime := new(MockRuntime)
	mockRuntime.On("Handle", mock.Anything, mock.MatchedBy(func(req runtime.EvaluationRequest) bool {
		return req.Command == runtime.CommandChat
	})).Return(chatRuntimeResponse("ASSISTANT", "Hello!"), nil)

	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(chatRequestBody(t)))
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeChat(w, req)

	res := w.Result()
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "application/json", res.Header.Get("Content-Type"))

	var chatResp map[string]any
	require.NoError(t, json.Unmarshal(body, &chatResp))
	output, ok := chatResp["output"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ASSISTANT", output["role"])
	assert.Equal(t, "Hello!", output["content"])

	mockRuntime.AssertExpectations(t)
}

func TestServeChat_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(chatRequestBody(t)))
	req.Header.Set("api-key", "wrong")
	w := httptest.NewRecorder()

	newMuEdHandler(nil, nil, "secret").ServeChat(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
}

func TestServeChat_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/chat", nil)
	w := httptest.NewRecorder()

	newMuEdHandler(nil, nil, "").ServeChat(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Result().StatusCode)
}

func TestServeChat_InvalidJSON(t *testing.T) {
	mockRuntime := new(MockRuntime)

	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeChat(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
	mockRuntime.AssertNotCalled(t, "Handle", mock.Anything, mock.Anything)
}

func TestServeChat_EmptyMessages(t *testing.T) {
	mockRuntime := new(MockRuntime)

	body, _ := json.Marshal(map[string]any{"messages": []any{}})
	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeChat(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
	mockRuntime.AssertNotCalled(t, "Handle", mock.Anything, mock.Anything)
}

func TestServeChat_RuntimeError(t *testing.T) {
	mockRuntime := new(MockRuntime)
	mockRuntime.On("Handle", mock.Anything, mock.Anything).
		Return(runtime.EvaluationResponse{}, errors.New("chat failed"))

	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(chatRequestBody(t)))
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeChat(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Result().StatusCode)
	mockRuntime.AssertExpectations(t)
}

// --- ServeChatHealth tests ---

func TestServeChatHealth_Success(t *testing.T) {
	mockRuntime := new(MockRuntime)
	mockRuntime.On("Handle", mock.Anything, runtime.EvaluationRequest{
		Command: runtime.CommandChatHealth,
		Data:    map[string]any{},
	}).Return(runtime.EvaluationResponse{
		"command": "chat/health",
		"result": map[string]any{
			"status": "OK",
			"capabilities": map[string]any{
				"chat": true,
			},
			"supportedLanguages":   []any{},
			"supportedModels":      []any{},
			"supportedAPIVersions": []any{},
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/chat/health", nil)
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeChatHealth(w, req)

	res := w.Result()
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "application/json", res.Header.Get("Content-Type"))

	var result map[string]any
	require.NoError(t, json.Unmarshal(raw, &result))
	assert.Equal(t, "OK", result["status"])

	mockRuntime.AssertExpectations(t)
}

func TestServeChatHealth_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/chat/health", nil)
	req.Header.Set("api-key", "wrong")
	w := httptest.NewRecorder()

	newMuEdHandler(nil, nil, "secret").ServeChatHealth(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
}

func TestServeChatHealth_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/chat/health", nil)
	w := httptest.NewRecorder()

	newMuEdHandler(nil, nil, "").ServeChatHealth(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Result().StatusCode)
}

func TestServeChatHealth_RuntimeError(t *testing.T) {
	mockRuntime := new(MockRuntime)
	mockRuntime.On("Handle", mock.Anything, mock.Anything).
		Return(runtime.EvaluationResponse{}, errors.New("worker unavailable"))

	req := httptest.NewRequest(http.MethodGet, "/chat/health", nil)
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeChatHealth(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Result().StatusCode)
	mockRuntime.AssertExpectations(t)
}
