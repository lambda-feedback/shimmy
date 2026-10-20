package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lambda-feedback/shimmy/internal/execution/supervisor"
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

func chatRuntimeResponse(role, content string) runtime.ChatResponse {
	return runtime.ChatResponse{
		Data: map[string]any{
			"command": "chat",
			"result": map[string]any{
				"output": map[string]any{
					"role":    role,
					"content": content,
				},
			},
		},
	}
}

// --- ServeChat tests ---

func TestServeChat_Success(t *testing.T) {
	mockRuntime := new(MockRuntime)
	mockRuntime.On("Chat", mock.Anything, mock.Anything).
		Return(chatRuntimeResponse("ASSISTANT", "Hello!"), nil)

	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(chatRequestBody(t)))
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeChat(w, req)

	res := w.Result()
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "application/json", res.Header.Get("Content-Type"))
	assert.Equal(t, "0.1.0", res.Header.Get("X-Api-Version"))

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
	mockRuntime.AssertNotCalled(t, "Chat", mock.Anything, mock.Anything)
}

func TestServeChat_EmptyMessages(t *testing.T) {
	mockRuntime := new(MockRuntime)

	body, _ := json.Marshal(map[string]any{"messages": []any{}})
	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeChat(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
	mockRuntime.AssertNotCalled(t, "Chat", mock.Anything, mock.Anything)
}

func TestServeChat_RuntimeError(t *testing.T) {
	mockRuntime := new(MockRuntime)
	mockRuntime.On("Chat", mock.Anything, mock.Anything).
		Return(runtime.ChatResponse{}, errors.New("chat failed"))

	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(chatRequestBody(t)))
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeChat(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Result().StatusCode)
	mockRuntime.AssertExpectations(t)
}

func TestServeChat_RetryableErrorsReturnFailedDependency(t *testing.T) {
	cases := []struct {
		name  string
		err   error
		code  string
		title string
	}{
		{
			name:  "upstream timeout",
			err:   &supervisor.UpstreamError{Message: "Please try again.", Timeout: true},
			code:  "UPSTREAM_TIMEOUT",
			title: "Evaluation timed out",
		},
		{
			name:  "upstream unavailable",
			err:   &supervisor.UpstreamError{Message: "Please try again."},
			code:  "UPSTREAM_UNAVAILABLE",
			title: "Evaluation service unavailable",
		},
		{
			name:  "function timeout",
			err:   &supervisor.FunctionTimeoutError{Message: "Please try again."},
			code:  "FUNCTION_TIMEOUT",
			title: "Evaluation timed out",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockRuntime := new(MockRuntime)
			mockRuntime.On("Chat", mock.Anything, mock.Anything).
				Return(runtime.ChatResponse{}, fmt.Errorf("error sending data: %w", tc.err))

			req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(chatRequestBody(t)))
			w := httptest.NewRecorder()

			newMuEdHandler(nil, mockRuntime, "").ServeChat(w, req)

			var body map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

			assert.Equal(t, http.StatusFailedDependency, w.Result().StatusCode)
			assert.Equal(t, tc.code, body["code"])
			assert.Equal(t, tc.title, body["title"])
			assert.Equal(t, "Please try again.", body["message"])
			assert.Equal(t, map[string]any{"retryable": true}, body["details"])
			mockRuntime.AssertExpectations(t)
		})
	}
}

// --- ServeChatHealth tests ---

func TestServeChatHealth_Success(t *testing.T) {
	mockRuntime := new(MockRuntime)
	mockRuntime.On("ChatHealth", mock.Anything).Return(runtime.ChatResponse{
		Data: map[string]any{
			"command": "chat/health",
			"result": map[string]any{
				"status":               "OK",
				"capabilities":         map[string]any{"supportsChat": true},
				"supportedLanguages":   []any{},
				"supportedModels":      []any{},
				"supportedAPIVersions": []any{},
			},
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
	assert.Equal(t, "0.1.0", res.Header.Get("X-Api-Version"))

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

func TestServeChatHealth_Unavailable(t *testing.T) {
	mockRuntime := new(MockRuntime)
	mockRuntime.On("ChatHealth", mock.Anything).Return(runtime.ChatResponse{
		Data: map[string]any{
			"result": map[string]any{
				"status": "UNAVAILABLE",
			},
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/chat/health", nil)
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeChatHealth(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Result().StatusCode)
	mockRuntime.AssertExpectations(t)
}

func TestServeChatHealth_RuntimeError(t *testing.T) {
	mockRuntime := new(MockRuntime)
	mockRuntime.On("ChatHealth", mock.Anything).
		Return(runtime.ChatResponse{}, errors.New("worker unavailable"))

	req := httptest.NewRequest(http.MethodGet, "/chat/health", nil)
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeChatHealth(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Result().StatusCode)
	mockRuntime.AssertExpectations(t)
}

// --- Version header tests (ServeChat) ---

func TestServeChat_AbsentVersionHeader(t *testing.T) {
	mockRuntime := new(MockRuntime)
	mockRuntime.On("Chat", mock.Anything, mock.Anything).
		Return(chatRuntimeResponse("ASSISTANT", "hi"), nil)

	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(chatRequestBody(t)))
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeChat(w, req)

	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "0.1.0", res.Header.Get("X-Api-Version"))
}

func TestServeChat_SupportedVersionHeader(t *testing.T) {
	mockRuntime := new(MockRuntime)
	mockRuntime.On("Chat", mock.Anything, mock.Anything).
		Return(chatRuntimeResponse("ASSISTANT", "hi"), nil)

	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(chatRequestBody(t)))
	req.Header.Set("X-Api-Version", "0.1.0")
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeChat(w, req)

	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "0.1.0", res.Header.Get("X-Api-Version"))
}

func TestServeChat_UnsupportedVersionHeader(t *testing.T) {
	mockRuntime := new(MockRuntime)

	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(chatRequestBody(t)))
	req.Header.Set("X-Api-Version", "99.0.0")
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeChat(w, req)

	res := w.Result()
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)

	assert.Equal(t, http.StatusNotAcceptable, res.StatusCode)
	assert.Equal(t, "0.1.0", res.Header.Get("X-Api-Version"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	assert.Equal(t, "VERSION_NOT_SUPPORTED", body["code"])
	details, ok := body["details"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "99.0.0", details["requestedVersion"])

	mockRuntime.AssertNotCalled(t, "Chat", mock.Anything, mock.Anything)
}

// --- Version header tests (ServeChatHealth) ---

func TestServeChatHealth_AbsentVersionHeader(t *testing.T) {
	mockRuntime := new(MockRuntime)
	mockRuntime.On("ChatHealth", mock.Anything).Return(runtime.ChatResponse{
		Data: map[string]any{
			"command": "chat/health",
			"result": map[string]any{
				"status":               "OK",
				"capabilities":         map[string]any{"supportsChat": true},
				"supportedLanguages":   []any{},
				"supportedModels":      []any{},
				"supportedAPIVersions": []any{},
			},
		},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/chat/health", nil)
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeChatHealth(w, req)

	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "0.1.0", res.Header.Get("X-Api-Version"))
	mockRuntime.AssertExpectations(t)
}

func TestServeChatHealth_UnsupportedVersionHeader(t *testing.T) {
	mockRuntime := new(MockRuntime)

	req := httptest.NewRequest(http.MethodGet, "/chat/health", nil)
	req.Header.Set("X-Api-Version", "99.0.0")
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeChatHealth(w, req)

	res := w.Result()
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)

	assert.Equal(t, http.StatusNotAcceptable, res.StatusCode)
	assert.Equal(t, "0.1.0", res.Header.Get("X-Api-Version"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	assert.Equal(t, "VERSION_NOT_SUPPORTED", body["code"])

	mockRuntime.AssertNotCalled(t, "ChatHealth", mock.Anything)
}
