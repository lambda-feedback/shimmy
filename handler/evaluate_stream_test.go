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
	"github.com/lambda-feedback/shimmy/internal/server"
	"github.com/lambda-feedback/shimmy/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// --- helpers ---

func inertFactory() progress.Factory {
	return progress.NewHTTPFactory(progress.HTTPFactoryParams{Log: zap.NewNop()})
}

func newStreamHandler(h runtime.Handler, pf progress.Factory, opts progress.StreamConfig) *MuEdHandler {
	if pf == nil {
		pf = inertFactory()
	}
	return &MuEdHandler{
		handler:          h,
		config:           config.Config{Progress: progress.Config{Stream: opts}},
		log:              zap.NewNop(),
		progressFactory:  pf,
		streamingCapable: true,
	}
}

func sseRequest(t *testing.T, body []byte) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(body))
	req.Header.Set("Accept", "text/event-stream")
	// SSE streaming is only offered on µEd versions whose contract declares it.
	req.Header.Set("X-Api-Version", "0.1.1-dev")
	return req
}

type sseFrame struct {
	event string
	data  map[string]any
}

// parseSSEAll returns every non-comment frame in order.
func parseSSEAll(t *testing.T, raw string) []sseFrame {
	t.Helper()
	var frames []sseFrame
	for _, block := range strings.Split(strings.TrimSpace(raw), "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" || strings.HasPrefix(block, ":") {
			continue
		}
		var f sseFrame
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "event: "):
				f.event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				f.data = map[string]any{}
				require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &f.data))
			}
		}
		frames = append(frames, f)
	}
	return frames
}

// parseSSE returns (eventName, decoded data) of the single terminal frame.
func parseSSE(t *testing.T, raw string) (string, map[string]any) {
	t.Helper()
	var event string
	var data map[string]any
	for _, block := range strings.Split(strings.TrimSpace(raw), "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" || strings.HasPrefix(block, ":") {
			continue
		}
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "event: "):
				event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &data))
			}
		}
	}
	return event, data
}

func previewBody(t *testing.T) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"submission": map[string]any{
			"type":    "MATH",
			"content": map[string]any{"expression": "x^2"},
		},
		"preSubmissionFeedback": map[string]any{"enabled": true},
	})
	require.NoError(t, err)
	return b
}

// --- tests ---

func TestServeEvaluate_SSE_Success(t *testing.T) {
	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).
		Return(evalHandlerResponse(true, "Well done"))

	req := sseRequest(t, mathEvalBody(t))
	req.Header.Set(muEdRequestIDHeader, "corr-sse")
	w := httptest.NewRecorder()

	newStreamHandler(mockHandler, nil, progress.StreamConfig{Enabled: true}).ServeEvaluate(w, req)

	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "text/event-stream", res.Header.Get("Content-Type"))
	assert.Equal(t, "corr-sse", res.Header.Get(muEdRequestIDHeader))
	assert.Empty(t, res.Header.Get("Content-Length"))

	event, data := parseSSE(t, w.Body.String())
	assert.Equal(t, "completed", event)
	assert.NotContains(t, data, "command", "the standardised terminal frame drops the command key")

	fb, ok := data["feedback"].([]any)
	require.True(t, ok, "feedback should be an array: %v", data["feedback"])
	require.Len(t, fb, 1)
	assert.Equal(t, "Well done", fb[0].(map[string]any)["message"])

	_, ok = data["steps"].([]any)
	assert.True(t, ok, "steps should always be present as an array")
}

func TestServeEvaluate_SSE_StreamsLiveStepFrames(t *testing.T) {
	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			ctx := args.Get(0).(context.Context)
			// Shim lifecycle markers, emitted twice as the per-case loop
			// would; only the first of each reaches the wire. Worker
			// sub-steps come in as StageEvaluating and are never collapsed.
			progress.Emit(ctx, progress.Event{Stage: progress.StagePreparing, Message: "Preparing…"})
			progress.Emit(ctx, progress.Event{Stage: progress.StageStarting, Message: "Starting…"})
			progress.Emit(ctx, progress.Event{Stage: progress.StageEvaluating, Message: "Parsing response and answer..."})
			progress.Emit(ctx, progress.Event{Stage: progress.StageEvaluating, Message: "Comparing sets for equivalence..."})
			progress.Emit(ctx, progress.Event{Stage: progress.StagePreparing, Message: "Preparing…"})
			progress.Emit(ctx, progress.Event{Stage: progress.StageStarting, Message: "Starting…"})
		}).
		Return(evalHandlerResponse(true, "Well done"))

	w := httptest.NewRecorder()
	newStreamHandler(mockHandler, nil, progress.StreamConfig{Enabled: true}).
		ServeEvaluate(w, sseRequest(t, mathEvalBody(t)))

	frames := parseSSEAll(t, w.Body.String())
	var events []string
	for _, f := range frames {
		events = append(events, f.event)
	}
	assert.Equal(t, []string{"preparing", "starting", "evaluating", "evaluating", "completed"}, events)

	// A live frame's data is one step object, identical in shape to an
	// element of the terminal frame's steps[].
	assert.Equal(t, "preparing", frames[0].data["stage"])
	assert.Equal(t, "Parsing response and answer...", frames[2].data["message"])

	steps, ok := frames[4].data["steps"].([]any)
	require.True(t, ok)
	require.Len(t, steps, 4)
	assert.Equal(t, "preparing", steps[0].(map[string]any)["stage"])
	assert.Equal(t, "starting", steps[1].(map[string]any)["stage"])
	assert.Equal(t, "evaluating", steps[3].(map[string]any)["stage"])
}

func TestServeEvaluate_SSE_Preview(t *testing.T) {
	previewResp := runtime.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: mustMarshal(t, map[string]any{
			"command": "preview",
			"result":  map[string]any{"preview": map[string]any{"feedback": "looks right"}},
		}),
	}
	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).Return(previewResp)

	w := httptest.NewRecorder()
	newStreamHandler(mockHandler, nil, progress.StreamConfig{Enabled: true}).ServeEvaluate(w, sseRequest(t, previewBody(t)))

	event, data := parseSSE(t, w.Body.String())
	assert.Equal(t, "completed", event)
	assert.NotContains(t, data, "command", "the standardised terminal frame drops the command key")
	fb := data["feedback"].([]any)
	require.Len(t, fb, 1)
	_, ok := fb[0].(map[string]any)["preSubmissionFeedback"]
	assert.True(t, ok, "expected preSubmissionFeedback wrapper, got %v", fb[0])
}

func TestServeEvaluate_SSE_WorkerNon200_BecomesFailedFrameAt200(t *testing.T) {
	errorBody := mustMarshal(t, map[string]any{"error": map[string]any{"message": "boom"}})
	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).Return(runtime.Response{
		StatusCode: http.StatusInternalServerError,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       errorBody,
	})

	w := httptest.NewRecorder()
	newStreamHandler(mockHandler, nil, progress.StreamConfig{Enabled: true}).ServeEvaluate(w, sseRequest(t, mathEvalBody(t)))

	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode, "the stream stays 200; failure is in-band")
	assert.Equal(t, "text/event-stream", res.Header.Get("Content-Type"))

	event, data := parseSSE(t, w.Body.String())
	assert.Equal(t, "failed", event)
	assert.Nil(t, data["feedback"])
	errObj, ok := data["error"].(map[string]any)
	require.True(t, ok, "error should be an ErrorResponse object, got %T", data["error"])
	assert.NotEmpty(t, errObj["title"])
	assert.Equal(t, "boom", errObj["message"])
	assert.Contains(t, errObj["trace"], "boom")
	assert.NotContains(t, data, "message", "failure detail now lives in the error object, not a top-level message")
}

func TestServeEvaluate_SSE_UnparseableWorkerResponse_FailedFrame(t *testing.T) {
	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).Return(runtime.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte("not json"),
	})

	w := httptest.NewRecorder()
	newStreamHandler(mockHandler, nil, progress.StreamConfig{Enabled: true}).ServeEvaluate(w, sseRequest(t, mathEvalBody(t)))

	assert.Equal(t, http.StatusOK, w.Result().StatusCode)
	event, data := parseSSE(t, w.Body.String())
	assert.Equal(t, "failed", event)
	assert.Nil(t, data["feedback"])
	errObj, ok := data["error"].(map[string]any)
	require.True(t, ok, "error should be an ErrorResponse object, got %T", data["error"])
	assert.NotEmpty(t, errObj["title"])
}

func TestServeEvaluate_SSE_CapabilityDisabled_FallsBackToJSON(t *testing.T) {
	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).
		Return(evalHandlerResponse(true, "Well done"))

	h := newStreamHandler(mockHandler, nil, progress.StreamConfig{Enabled: true})
	h.streamingCapable = false

	w := httptest.NewRecorder()
	h.ServeEvaluate(w, sseRequest(t, mathEvalBody(t)))

	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "application/json", res.Header.Get("Content-Type"))

	var feedback []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &feedback))
	require.Len(t, feedback, 1)
	assert.Equal(t, "Well done", feedback[0]["message"])
}

func TestServeEvaluate_SSE_StreamConfigDisabled_FallsBackToJSON(t *testing.T) {
	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).
		Return(evalHandlerResponse(true, "Well done"))

	w := httptest.NewRecorder()
	newStreamHandler(mockHandler, nil, progress.StreamConfig{Enabled: false}).ServeEvaluate(w, sseRequest(t, mathEvalBody(t)))

	assert.Equal(t, "application/json", w.Result().Header.Get("Content-Type"))
}

func TestServeEvaluate_SSE_NoAcceptHeader_Unchanged(t *testing.T) {
	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).
		Return(evalHandlerResponse(true, "Well done"))

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(mathEvalBody(t)))
	w := httptest.NewRecorder()
	newStreamHandler(mockHandler, nil, progress.StreamConfig{Enabled: true}).ServeEvaluate(w, req)

	assert.Equal(t, "application/json", w.Result().Header.Get("Content-Type"))
}

func TestServeEvaluate_SSE_WithCallbackUrl_BothDelivered(t *testing.T) {
	srv, received := newProgressCallbackServer(t, nil)

	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).
		Return(evalHandlerResponse(true, "Well done"))

	req := sseRequest(t, mathEvalBodyWithCallback(t, srv.URL))
	req.Header.Set(muEdRequestIDHeader, "corr-both")
	w := httptest.NewRecorder()

	newStreamHandler(mockHandler, newProgressFactory(t, time.Second), progress.StreamConfig{Enabled: true}).
		ServeEvaluate(w, req)

	// SSE side
	event, data := parseSSE(t, w.Body.String())
	assert.Equal(t, "completed", event)
	assert.NotContains(t, data, "command", "the standardised terminal frame drops the command key")

	// callbackUrl side
	require.Len(t, *received, 1)
	evt := (*received)[0]
	assert.Equal(t, "corr-both", evt["correlationId"])
	assert.Equal(t, "completed", evt["stage"])
}

func TestServeEvaluate_SSE_AuthFailure_StillHTTPError(t *testing.T) {
	mockHandler := new(MockHandler)
	h := newStreamHandler(mockHandler, nil, progress.StreamConfig{Enabled: true})
	h.config.Auth.Key = "secret"

	w := httptest.NewRecorder()
	h.ServeEvaluate(w, sseRequest(t, mathEvalBody(t)))

	assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
	assert.NotEqual(t, "text/event-stream", w.Result().Header.Get("Content-Type"))
	mockHandler.AssertNotCalled(t, "Handle", mock.Anything, mock.Anything)
}

func TestServeEvaluate_SSE_UnsupportedVersion_StillHTTPError(t *testing.T) {
	mockHandler := new(MockHandler)
	h := newStreamHandler(mockHandler, nil, progress.StreamConfig{Enabled: true})

	req := sseRequest(t, mathEvalBody(t))
	req.Header.Set(muEdVersionHeader, "99.0.0")
	w := httptest.NewRecorder()
	h.ServeEvaluate(w, req)

	assert.Equal(t, http.StatusNotAcceptable, w.Result().StatusCode)
	mockHandler.AssertNotCalled(t, "Handle", mock.Anything, mock.Anything)
}

func TestServeEvaluate_SSE_Heartbeat(t *testing.T) {
	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).
		Return(evalHandlerResponse(true, "Well done")).
		After(1200 * time.Millisecond)

	h := newStreamHandler(mockHandler, nil, progress.StreamConfig{Enabled: true, HeartbeatSeconds: 1})
	srv := httptest.NewServer(http.HandlerFunc(h.ServeEvaluate))
	defer srv.Close()

	reqBody := bytes.NewReader(mathEvalBody(t))
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/evaluate", reqBody)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("X-Api-Version", "0.1.1-dev")

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

// TestServeEvaluate_SSE_ThroughOpenAPIMiddleware exercises the real serve-mode
// chain end to end: a live socket, the OpenAPI middleware (which must NOT
// buffer the stream), NormalizePath, and the streaming handler with a real
// flushable ResponseWriter.
func TestServeEvaluate_SSE_ThroughOpenAPIMiddleware(t *testing.T) {
	specs, err := server.LoadOpenAPISpecs()
	require.NoError(t, err)
	mw, err := server.OpenAPIMiddleware(specs, nil, zap.NewNop())
	require.NoError(t, err)

	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).
		Return(evalHandlerResponse(true, "Well done"))
	h := newStreamHandler(mockHandler, nil, progress.StreamConfig{Enabled: true})

	mux := http.NewServeMux()
	mux.HandleFunc("/evaluate", h.ServeEvaluate)
	srv := httptest.NewServer(mw(server.NormalizePath(mux)))
	defer srv.Close()

	// Spec-valid body: the OpenAPI middleware validates the request before
	// the streaming bypass, and the spec requires task.title.
	body := mustMarshal(t, map[string]any{
		"submission": map[string]any{
			"type":    "MATH",
			"content": map[string]any{"expression": "x^2"},
		},
		"task": map[string]any{
			"title":             "t",
			"referenceSolution": map[string]any{"expression": "x^2"},
		},
	})
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/evaluate", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Version", "0.1.1-dev")

	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Empty(t, resp.Header.Get("Content-Length"))

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	event, data := parseSSE(t, string(raw))
	assert.Equal(t, "completed", event)
	assert.NotContains(t, data, "command", "the standardised terminal frame drops the command key")
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}
