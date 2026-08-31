package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lambda-feedback/shimmy/config"
	"github.com/lambda-feedback/shimmy/internal/progress"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// --- helpers ---

func newChatStreamHandler(rt *MockRuntime, pf progress.Factory, opts progress.StreamConfig) *MuEdHandler {
	if pf == nil {
		pf = inertFactory()
	}
	return &MuEdHandler{
		runtime:          rt,
		config:           config.Config{Progress: progress.Config{Stream: opts}},
		log:              zap.NewNop(),
		progressFactory:  pf,
		streamingCapable: true,
	}
}

func chatSSERequest(t *testing.T, body []byte) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	req.Header.Set("Accept", "text/event-stream")
	return req
}

func chatBodyWithCallback(t *testing.T, callbackURL string) []byte {
	t.Helper()
	return mustMarshal(t, map[string]any{
		"messages":    []map[string]any{{"role": "USER", "content": "hello"}},
		"callbackUrl": callbackURL,
	})
}

// --- tests ---

func TestServeChat_SSE_Success(t *testing.T) {
	rt := new(MockRuntime)
	rt.On("Chat", mock.Anything, mock.Anything).
		Return(chatRuntimeResponse("ASSISTANT", "Here you go"), nil)

	req := chatSSERequest(t, chatRequestBody(t))
	req.Header.Set(muEdRequestIDHeader, "corr-chat")
	w := httptest.NewRecorder()

	newChatStreamHandler(rt, nil, progress.StreamConfig{Enabled: true}).ServeChat(w, req)

	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "text/event-stream", res.Header.Get("Content-Type"))
	assert.Equal(t, "corr-chat", res.Header.Get(muEdRequestIDHeader))
	assert.Empty(t, res.Header.Get("Content-Length"))

	event, data := parseSSE(t, w.Body.String())
	assert.Equal(t, "completed", event)
	assert.Equal(t, "chat", data["command"])
	if _, hasFeedback := data["feedback"]; hasFeedback {
		t.Errorf("chat frame must not carry a feedback key: %v", data)
	}
	out, ok := data["output"].(map[string]any)
	require.True(t, ok, "output should be an object: %v", data["output"])
	assert.Equal(t, "Here you go", out["content"])
	_, ok = data["steps"].([]any)
	assert.True(t, ok, "steps should always be present as an array")
}

func TestServeChat_SSE_StreamsThinkingFrames(t *testing.T) {
	rt := new(MockRuntime)
	rt.On("Chat", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			ctx := args.Get(0).(context.Context)
			progress.Emit(ctx, progress.Event{Stage: progress.StagePreparing, Message: "Preparing…"})
			progress.Emit(ctx, progress.Event{Stage: progress.StageStarting, Message: "Starting…"})
			progress.Emit(ctx, progress.Event{Stage: progress.StageThinking, Message: "Searching your notes…"})
			progress.Emit(ctx, progress.Event{Stage: progress.StageThinking, Message: "Drafting a reply…"})
		}).
		Return(chatRuntimeResponse("ASSISTANT", "Done"), nil)

	w := httptest.NewRecorder()
	newChatStreamHandler(rt, nil, progress.StreamConfig{Enabled: true}).
		ServeChat(w, chatSSERequest(t, chatRequestBody(t)))

	frames := parseSSEAll(t, w.Body.String())
	var events []string
	for _, f := range frames {
		events = append(events, f.event)
	}
	assert.Equal(t, []string{"preparing", "starting", "thinking", "thinking", "completed"}, events)
	assert.Equal(t, "Searching your notes…", frames[2].data["message"])

	steps := frames[4].data["steps"].([]any)
	require.Len(t, steps, 4)
	assert.Equal(t, "thinking", steps[3].(map[string]any)["stage"])
}

func TestServeChat_SSE_RuntimeError_BecomesFailedFrameAt200(t *testing.T) {
	rt := new(MockRuntime)
	rt.On("Chat", mock.Anything, mock.Anything).
		Return(chatRuntimeResponse("ASSISTANT", ""), assertAnError{})

	w := httptest.NewRecorder()
	newChatStreamHandler(rt, nil, progress.StreamConfig{Enabled: true}).
		ServeChat(w, chatSSERequest(t, chatRequestBody(t)))

	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode, "the stream stays 200; failure is in-band")

	event, data := parseSSE(t, w.Body.String())
	assert.Equal(t, "failed", event)
	assert.Nil(t, data["output"])
	assert.Contains(t, data["error"], "boom")
}

func TestServeChat_SSE_CapabilityDisabled_FallsBackToJSON(t *testing.T) {
	rt := new(MockRuntime)
	rt.On("Chat", mock.Anything, mock.Anything).
		Return(chatRuntimeResponse("ASSISTANT", "hi"), nil)

	h := newChatStreamHandler(rt, nil, progress.StreamConfig{Enabled: true})
	h.streamingCapable = false

	w := httptest.NewRecorder()
	h.ServeChat(w, chatSSERequest(t, chatRequestBody(t)))

	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "application/json", res.Header.Get("Content-Type"))

	var chatResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &chatResp))
	out := chatResp["output"].(map[string]any)
	assert.Equal(t, "hi", out["content"])
}

func TestServeChat_SSE_StreamConfigDisabled_FallsBackToJSON(t *testing.T) {
	rt := new(MockRuntime)
	rt.On("Chat", mock.Anything, mock.Anything).
		Return(chatRuntimeResponse("ASSISTANT", "hi"), nil)

	w := httptest.NewRecorder()
	newChatStreamHandler(rt, nil, progress.StreamConfig{Enabled: false}).
		ServeChat(w, chatSSERequest(t, chatRequestBody(t)))

	assert.Equal(t, "application/json", w.Result().Header.Get("Content-Type"))
}

func TestServeChat_SSE_NoAcceptHeader_Unchanged(t *testing.T) {
	rt := new(MockRuntime)
	rt.On("Chat", mock.Anything, mock.Anything).
		Return(chatRuntimeResponse("ASSISTANT", "hi"), nil)

	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(chatRequestBody(t)))
	w := httptest.NewRecorder()
	newChatStreamHandler(rt, nil, progress.StreamConfig{Enabled: true}).ServeChat(w, req)

	assert.Equal(t, "application/json", w.Result().Header.Get("Content-Type"))
}

func TestServeChat_SSE_WithCallbackUrl_BothDelivered(t *testing.T) {
	srv, received := newProgressCallbackServer(t, nil)

	rt := new(MockRuntime)
	rt.On("Chat", mock.Anything, mock.Anything).
		Return(chatRuntimeResponse("ASSISTANT", "Here you go"), nil)

	req := chatSSERequest(t, chatBodyWithCallback(t, srv.URL))
	req.Header.Set(muEdRequestIDHeader, "corr-chat-both")
	w := httptest.NewRecorder()

	newChatStreamHandler(rt, newProgressFactory(t, time.Second), progress.StreamConfig{Enabled: true}).
		ServeChat(w, req)

	event, data := parseSSE(t, w.Body.String())
	assert.Equal(t, "completed", event)
	assert.Equal(t, "chat", data["command"])

	require.Len(t, *received, 1)
	evt := (*received)[0]
	assert.Equal(t, "corr-chat-both", evt["correlationId"])
	assert.Equal(t, "completed", evt["stage"])
}

func TestServeChat_SSE_AuthFailure_StillHTTPError(t *testing.T) {
	rt := new(MockRuntime)
	h := newChatStreamHandler(rt, nil, progress.StreamConfig{Enabled: true})
	h.config.Auth.Key = "secret"

	w := httptest.NewRecorder()
	h.ServeChat(w, chatSSERequest(t, chatRequestBody(t)))

	assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
	assert.NotEqual(t, "text/event-stream", w.Result().Header.Get("Content-Type"))
	rt.AssertNotCalled(t, "Chat", mock.Anything, mock.Anything)
}

func TestServeChat_SSE_Heartbeat(t *testing.T) {
	rt := new(MockRuntime)
	rt.On("Chat", mock.Anything, mock.Anything).
		Return(chatRuntimeResponse("ASSISTANT", "hi"), nil).
		After(1200 * time.Millisecond)

	h := newChatStreamHandler(rt, nil, progress.StreamConfig{Enabled: true, HeartbeatSeconds: 1})
	srv := httptest.NewServer(http.HandlerFunc(h.ServeChat))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/chat", bytes.NewReader(chatRequestBody(t)))
	require.NoError(t, err)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var sawPing bool
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if strings.HasPrefix(line, ": ping") {
			sawPing = true
		}
		if strings.HasPrefix(line, "event: completed") {
			break
		}
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
	}
	assert.True(t, sawPing, "expected at least one heartbeat before the completed frame")
}

// assertAnError is an error whose message contains "boom", for the failure-path test.
type assertAnError struct{}

func (assertAnError) Error() string { return "chat failed: boom" }
