package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lambda-feedback/shimmy/config"
	"github.com/lambda-feedback/shimmy/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// markerAdapter is a synthetic MuEdAdapter whose outputs are easy to recognise,
// used to prove the handler dispatches to the resolved version's adapter rather
// than to hard-coded v0.1.0 logic.
type markerAdapter struct{}

func (markerAdapter) Version() string { return "9.9.9" }
func (markerAdapter) DecodeEvaluate([]byte) (map[string]any, runtime.Command, error) {
	return map[string]any{"marker": "decoded"}, runtime.CommandEvaluate, nil
}
func (markerAdapter) EncodeEvaluateFeedback(runtime.Command, map[string]any) ([]map[string]any, error) {
	return []map[string]any{{"marker": "feedback-9.9.9"}}, nil
}
func (markerAdapter) EncodeHealth(map[string]any) map[string]any {
	return map[string]any{"marker": "health-9.9.9"}
}
func (markerAdapter) DecodeChat([]byte) (map[string]any, error) {
	return map[string]any{"marker": "chat"}, nil
}
func (markerAdapter) EncodeChat(map[string]any) (map[string]any, error) {
	return map[string]any{"marker": "chat-9.9.9"}, nil
}
func (markerAdapter) EncodeChatHealth(map[string]any) map[string]any {
	return map[string]any{"marker": "chat-health-9.9.9"}
}

func newMuEdHandlerWithRegistry(h runtime.Handler, r runtime.Runtime, reg *runtime.MuEdRegistry) *MuEdHandler {
	return &MuEdHandler{
		handler:  h,
		runtime:  r,
		registry: reg,
		config:   config.Config{},
		log:      zap.NewNop(),
	}
}

// TestMuEdServeEvaluate_DispatchesToResolvedAdapter proves version dispatch:
// an X-Api-Version the injected registry supports is routed to that version's
// adapter, and the response echoes the resolved version.
func TestMuEdServeEvaluate_DispatchesToResolvedAdapter(t *testing.T) {
	reg := runtime.NewMuEdRegistry()
	reg.Register(markerAdapter{})

	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).Return(evalHandlerResponse(true, "ignored"))

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(mathEvalBody(t)))
	req.Header.Set("X-Api-Version", "9.9.9")
	w := httptest.NewRecorder()

	newMuEdHandlerWithRegistry(mockHandler, nil, reg).ServeEvaluate(w, req)

	res := w.Result()
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "9.9.9", res.Header.Get("X-Api-Version"))

	var feedback []map[string]any
	require.NoError(t, json.Unmarshal(raw, &feedback))
	require.Len(t, feedback, 1)
	assert.Equal(t, "feedback-9.9.9", feedback[0]["marker"])

	mockHandler.AssertExpectations(t)
}

// TestMuEdServeEvaluate_VersionParity runs the happy path with both an absent
// header and an explicit supported header and asserts identical output.
func TestMuEdServeEvaluate_VersionParity(t *testing.T) {
	run := func(setHeader bool) (int, string, []map[string]any) {
		mockHandler := new(MockHandler)
		mockHandler.On("Handle", mock.Anything, mock.Anything).
			Return(evalHandlerResponse(true, "Well done"))

		req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(mathEvalBody(t)))
		if setHeader {
			req.Header.Set("X-Api-Version", "0.1.0")
		}
		w := httptest.NewRecorder()
		newMuEdHandler(mockHandler, nil, "").ServeEvaluate(w, req)

		res := w.Result()
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		var fb []map[string]any
		require.NoError(t, json.Unmarshal(raw, &fb))
		return res.StatusCode, res.Header.Get("X-Api-Version"), fb
	}

	absentCode, absentVer, absentFb := run(false)
	explicitCode, explicitVer, explicitFb := run(true)

	assert.Equal(t, http.StatusOK, absentCode)
	assert.Equal(t, absentCode, explicitCode)
	assert.Equal(t, "0.1.0", absentVer)
	assert.Equal(t, absentVer, explicitVer)
	assert.Equal(t, absentFb, explicitFb)
}

// TestMuEdServeChat_DispatchesToResolvedAdapter is the chat-side counterpart.
func TestMuEdServeChat_DispatchesToResolvedAdapter(t *testing.T) {
	reg := runtime.NewMuEdRegistry()
	reg.Register(markerAdapter{})

	mockRuntime := new(MockRuntime)
	mockRuntime.On("Chat", mock.Anything, mock.Anything).
		Return(chatRuntimeResponse("ASSISTANT", "ignored"), nil)

	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(chatRequestBody(t)))
	req.Header.Set("X-Api-Version", "9.9.9")
	w := httptest.NewRecorder()

	newMuEdHandlerWithRegistry(nil, mockRuntime, reg).ServeChat(w, req)

	res := w.Result()
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "9.9.9", res.Header.Get("X-Api-Version"))

	var resp map[string]any
	require.NoError(t, json.Unmarshal(raw, &resp))
	assert.Equal(t, "chat-9.9.9", resp["marker"])

	mockRuntime.AssertExpectations(t)
}
