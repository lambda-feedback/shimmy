package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/lambda-feedback/shimmy/config"
	"github.com/lambda-feedback/shimmy/internal/progress"
	"github.com/lambda-feedback/shimmy/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// --- Mock runtime ---

type MockRuntime struct {
	mock.Mock
}

func (m *MockRuntime) Handle(ctx context.Context, req runtime.EvaluationRequest) (runtime.EvaluationResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(runtime.EvaluationResponse), args.Error(1)
}

func (m *MockRuntime) Chat(ctx context.Context, req runtime.ChatRequest) (runtime.ChatResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(runtime.ChatResponse), args.Error(1)
}

func (m *MockRuntime) ChatHealth(ctx context.Context) (runtime.ChatResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(runtime.ChatResponse), args.Error(1)
}

func (m *MockRuntime) Start(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

func (m *MockRuntime) Shutdown(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

// --- Helpers ---

// newMuEdHandler builds a handler with a default, inert progress factory:
// since none of the existing tests set callbackUrl in the request body,
// NewReporter always returns (nil, nil) and behavior is unchanged. Tests
// that exercise progress reporting itself use newMuEdHandlerWithProgress.
func newMuEdHandler(h runtime.Handler, r runtime.Runtime, key string) *MuEdHandler {
	return newMuEdHandlerWithProgress(h, r, key, progress.NewHTTPFactory(progress.HTTPFactoryParams{
		Log: zap.NewNop(),
	}))
}

func newMuEdHandlerWithProgress(h runtime.Handler, r runtime.Runtime, key string, pf progress.Factory) *MuEdHandler {
	return &MuEdHandler{
		handler:         h,
		runtime:         r,
		config:          config.Config{Auth: config.AuthConfig{Key: key}},
		log:             zap.NewNop(),
		progressFactory: pf,
	}
}

func mathEvalBody(t *testing.T) []byte {
	t.Helper()
	return mathEvalBodyWithCallback(t, "")
}

func mathEvalBodyWithCallback(t *testing.T, callbackURL string) []byte {
	t.Helper()
	body := map[string]any{
		"submission": map[string]any{
			"type":    "MATH",
			"content": map[string]any{"expression": "x^2"},
		},
		"task": map[string]any{
			"referenceSolution": map[string]any{
				"expression": "x^2",
			},
		},
	}
	if callbackURL != "" {
		body["callbackUrl"] = callbackURL
	}
	b, err := json.Marshal(body)
	require.NoError(t, err)
	return b
}

func evalHandlerResponse(isCorrect bool, feedback string) runtime.Response {
	body, _ := json.Marshal(map[string]any{
		"command": "eval",
		"result": map[string]any{
			"is_correct": isCorrect,
			"feedback":   feedback,
		},
	})
	return runtime.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       body,
	}
}

// --- ServeEvaluate tests ---

func TestMuEdServeEvaluate_Success(t *testing.T) {
	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).
		Return(evalHandlerResponse(true, "Well done"))

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(mathEvalBody(t)))
	w := httptest.NewRecorder()

	newMuEdHandler(mockHandler, nil, "").ServeEvaluate(w, req)

	res := w.Result()
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "application/json", res.Header.Get("Content-Type"))
	assert.Equal(t, "0.1.0", res.Header.Get("X-Api-Version"))

	var feedback []map[string]any
	require.NoError(t, json.Unmarshal(body, &feedback))
	require.Len(t, feedback, 1)
	assert.Equal(t, 1.0, feedback[0]["awardedPoints"])
	assert.Equal(t, "Well done", feedback[0]["message"])
	assert.Contains(t, string(body), `"responseLatex":null`)
	assert.Contains(t, string(body), `"responseSimplified":null`)

	mockHandler.AssertExpectations(t)
}

func TestMuEdServeEvaluate_LegacyBodyForwarded(t *testing.T) {
	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.MatchedBy(func(r runtime.Request) bool {
		var body map[string]any
		if err := json.Unmarshal(r.Body, &body); err != nil {
			return false
		}
		return body["response"] == "x^2" &&
			body["answer"] == "x^2" &&
			r.Header.Get("Command") == "eval"
	})).Return(evalHandlerResponse(true, "Correct"))

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(mathEvalBody(t)))
	w := httptest.NewRecorder()

	newMuEdHandler(mockHandler, nil, "").ServeEvaluate(w, req)

	assert.Equal(t, http.StatusOK, w.Result().StatusCode)
	mockHandler.AssertExpectations(t)
}

func TestMuEdServeEvaluate_Preview(t *testing.T) {
	previewBody, _ := json.Marshal(map[string]any{
		"submission": map[string]any{
			"type":    "MATH",
			"content": map[string]any{"expression": "x^2"},
		},
		"preSubmissionFeedback": map[string]any{"enabled": true},
	})

	previewResult := map[string]any{"preview": map[string]any{"latex": "x^{2}"}}
	respBody, _ := json.Marshal(map[string]any{
		"command": "preview",
		"result":  previewResult,
	})
	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.MatchedBy(func(r runtime.Request) bool {
		return r.Header.Get("Command") == "preview"
	})).Return(runtime.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       respBody,
	})

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(previewBody))
	w := httptest.NewRecorder()

	newMuEdHandler(mockHandler, nil, "").ServeEvaluate(w, req)

	res := w.Result()
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)

	assert.Equal(t, http.StatusOK, res.StatusCode)

	var feedback []map[string]any
	require.NoError(t, json.Unmarshal(raw, &feedback))
	require.Len(t, feedback, 1)
	assert.NotNil(t, feedback[0]["preSubmissionFeedback"])

	mockHandler.AssertExpectations(t)
}

func TestMuEdServeEvaluate_Unauthorized(t *testing.T) {
	mockHandler := new(MockHandler)

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(mathEvalBody(t)))
	req.Header.Set("api-key", "wrong")
	w := httptest.NewRecorder()

	newMuEdHandler(mockHandler, nil, "secret").ServeEvaluate(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
	mockHandler.AssertNotCalled(t, "Handle", mock.Anything, mock.Anything)
}

func TestMuEdServeEvaluate_MethodNotAllowed(t *testing.T) {
	mockHandler := new(MockHandler)

	req := httptest.NewRequest(http.MethodGet, "/evaluate", nil)
	w := httptest.NewRecorder()

	newMuEdHandler(mockHandler, nil, "").ServeEvaluate(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Result().StatusCode)
	mockHandler.AssertNotCalled(t, "Handle", mock.Anything, mock.Anything)
}

func TestMuEdServeEvaluate_InvalidJSON(t *testing.T) {
	mockHandler := new(MockHandler)

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()

	newMuEdHandler(mockHandler, nil, "").ServeEvaluate(w, req)

	res := w.Result()
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)

	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	assert.Equal(t, "0.1.0", res.Header.Get("X-Api-Version"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	assert.Equal(t, "VALIDATION_ERROR", body["code"])

	mockHandler.AssertNotCalled(t, "Handle", mock.Anything, mock.Anything)
}

func TestMuEdServeEvaluate_MissingReferenceSolution(t *testing.T) {
	mockHandler := new(MockHandler)

	reqBody, _ := json.Marshal(map[string]any{
		"submission": map[string]any{
			"type":    "MATH",
			"content": map[string]any{"expression": "x^2"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	newMuEdHandler(mockHandler, nil, "").ServeEvaluate(w, req)

	res := w.Result()
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)

	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	assert.Equal(t, "0.1.0", res.Header.Get("X-Api-Version"))

	var errBody map[string]any
	require.NoError(t, json.Unmarshal(raw, &errBody))
	assert.Equal(t, "VALIDATION_ERROR", errBody["code"])

	mockHandler.AssertNotCalled(t, "Handle", mock.Anything, mock.Anything)
}

func TestMuEdServeEvaluate_WorkerErrorForwarded(t *testing.T) {
	errorBody, _ := json.Marshal(map[string]any{
		"error": map[string]any{"message": "evaluation failed"},
	})
	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).Return(runtime.Response{
		StatusCode: http.StatusInternalServerError,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       errorBody,
	})

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(mathEvalBody(t)))
	w := httptest.NewRecorder()

	newMuEdHandler(mockHandler, nil, "").ServeEvaluate(w, req)

	res := w.Result()
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	assert.Equal(t, "0.1.0", res.Header.Get("X-Api-Version"))
	assert.Equal(t, errorBody, bytes.TrimRight(raw, "\n"))
}

// --- Progress callback tests (ServeEvaluate) ---

// newProgressCallbackServer spins up a fake progress-callback receiver
// that records every decoded request body it receives.
func newProgressCallbackServer(t *testing.T, handlerFn http.HandlerFunc) (*httptest.Server, *[]map[string]any) {
	t.Helper()

	var mu sync.Mutex
	var received []map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		received = append(received, body)
		mu.Unlock()

		if handlerFn != nil {
			handlerFn(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	return srv, &received
}

// newProgressFactory builds a factory with SSRF protection relaxed: these
// tests use httptest.NewServer (a loopback address) to stand in for the
// caller's real, non-loopback callback receiver, so the default
// private-network guard would otherwise reject every delivery here. The
// guard itself is covered directly in internal/progress.
func newProgressFactory(t *testing.T, timeout time.Duration) progress.Factory {
	t.Helper()
	return progress.NewHTTPFactory(progress.HTTPFactoryParams{
		Config: progress.Config{CallbackTimeout: timeout, AllowPrivateNetworks: true},
		Log:    zap.NewNop(),
	})
}

func TestMuEdServeEvaluate_ProgressCallback_Success(t *testing.T) {
	srv, received := newProgressCallbackServer(t, nil)

	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).
		Return(evalHandlerResponse(true, "Well done"))

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(mathEvalBodyWithCallback(t, srv.URL)))
	req.Header.Set(muEdRequestIDHeader, "corr-1")
	w := httptest.NewRecorder()

	newMuEdHandlerWithProgress(mockHandler, nil, "", newProgressFactory(t, time.Second)).ServeEvaluate(w, req)

	assert.Equal(t, http.StatusOK, w.Result().StatusCode)

	require.Len(t, *received, 1)
	evt := (*received)[0]
	assert.Equal(t, "corr-1", evt["correlationId"])
	assert.Equal(t, "completed", evt["stage"])
	assert.Equal(t, "eval", evt["command"])

	data, ok := evt["data"].(map[string]any)
	require.True(t, ok, "expected data field on the completed event")
	feedback, ok := data["feedback"].([]any)
	require.True(t, ok, "expected data.feedback array")
	require.Len(t, feedback, 1)
	item, ok := feedback[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Well done", item["message"])
	assert.Equal(t, 1.0, item["awardedPoints"])
}

func TestMuEdServeEvaluate_ProgressCallback_Failure(t *testing.T) {
	srv, received := newProgressCallbackServer(t, nil)

	errorBody, _ := json.Marshal(map[string]any{
		"error": map[string]any{"message": "boom"},
	})
	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).Return(runtime.Response{
		StatusCode: http.StatusInternalServerError,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       errorBody,
	})

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(mathEvalBodyWithCallback(t, srv.URL)))
	req.Header.Set(muEdRequestIDHeader, "corr-2")
	w := httptest.NewRecorder()

	newMuEdHandlerWithProgress(mockHandler, nil, "", newProgressFactory(t, time.Second)).ServeEvaluate(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Result().StatusCode)

	require.Len(t, *received, 1)
	evt := (*received)[0]
	assert.Equal(t, "corr-2", evt["correlationId"])
	assert.Equal(t, "failed", evt["stage"])
	assert.Equal(t, "boom", evt["message"])
	// error is the same ErrorResponse-shaped object as on the SSE frame.
	errObj, ok := evt["error"].(map[string]any)
	require.True(t, ok, "callback error should be an ErrorResponse object, got %T", evt["error"])
	assert.NotEmpty(t, errObj["title"])
	assert.Equal(t, "boom", errObj["message"])
}

func TestMuEdServeEvaluate_ProgressCallback_NoCallbackUrl_Unchanged(t *testing.T) {
	_, received := newProgressCallbackServer(t, nil)

	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).
		Return(evalHandlerResponse(true, "Well done"))

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(mathEvalBody(t)))
	w := httptest.NewRecorder()

	newMuEdHandlerWithProgress(mockHandler, nil, "", newProgressFactory(t, time.Second)).ServeEvaluate(w, req)

	assert.Equal(t, http.StatusOK, w.Result().StatusCode)
	assert.Empty(t, *received, "no callbackUrl in the request body should mean no callback requests")
}

func TestMuEdServeEvaluate_ProgressCallback_InvalidCallbackUrl_EvaluationStillSucceeds(t *testing.T) {
	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).
		Return(evalHandlerResponse(true, "Well done"))

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(mathEvalBodyWithCallback(t, "not-a-url")))
	w := httptest.NewRecorder()

	newMuEdHandlerWithProgress(mockHandler, nil, "", newProgressFactory(t, time.Second)).ServeEvaluate(w, req)

	res := w.Result()
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	assert.Equal(t, http.StatusOK, res.StatusCode)

	var feedback []map[string]any
	require.NoError(t, json.Unmarshal(body, &feedback))
	require.Len(t, feedback, 1)
}

func TestMuEdServeEvaluate_ProgressCallback_SlowReceiver_DoesNotBlockResponse(t *testing.T) {
	srv, _ := newProgressCallbackServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).
		Return(evalHandlerResponse(true, "Well done"))

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(mathEvalBodyWithCallback(t, srv.URL)))
	req.Header.Set(muEdRequestIDHeader, "corr-3")
	w := httptest.NewRecorder()

	start := time.Now()
	newMuEdHandlerWithProgress(mockHandler, nil, "", newProgressFactory(t, 20*time.Millisecond)).ServeEvaluate(w, req)
	elapsed := time.Since(start)

	assert.Equal(t, http.StatusOK, w.Result().StatusCode)
	assert.Less(t, elapsed, 150*time.Millisecond, "ServeEvaluate should return promptly, bounded by CallbackTimeout")
}

// --- ServeHealth tests ---

func TestMuEdServeHealth_Success(t *testing.T) {
	healthResult := map[string]any{"tests_passed": true, "successes": []any{}, "failures": []any{}, "errors": []any{}}
	mockRuntime := new(MockRuntime)
	mockRuntime.On("Handle", mock.Anything, runtime.EvaluationRequest{
		Command: runtime.CommandEvaluateHealth,
		Data:    map[string]any{},
	}).Return(runtime.EvaluationResponse{
		"command": "healthcheck",
		"result":  healthResult,
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/evaluate/health", nil)
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeHealth(w, req)

	res := w.Result()
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "application/json", res.Header.Get("Content-Type"))
	assert.Equal(t, "0.1.0", res.Header.Get("X-Api-Version"))

	var result map[string]any
	require.NoError(t, json.Unmarshal(raw, &result))
	assert.Equal(t, "OK", result["status"])
	caps, ok := result["capabilities"].(map[string]any)
	require.True(t, ok)
	versions, ok := caps["supportedAPIVersions"].([]any)
	require.True(t, ok)
	assert.Contains(t, versions, "0.1.0")

	mockRuntime.AssertExpectations(t)
}

func TestMuEdServeHealth_Unauthorized(t *testing.T) {
	mockRuntime := new(MockRuntime)

	req := httptest.NewRequest(http.MethodGet, "/evaluate/health", nil)
	req.Header.Set("api-key", "wrong")
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "secret").ServeHealth(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
	mockRuntime.AssertNotCalled(t, "Handle", mock.Anything, mock.Anything)
}

func TestMuEdServeHealth_MethodNotAllowed(t *testing.T) {
	mockRuntime := new(MockRuntime)

	req := httptest.NewRequest(http.MethodPost, "/evaluate/health", nil)
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeHealth(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Result().StatusCode)
	mockRuntime.AssertNotCalled(t, "Handle", mock.Anything, mock.Anything)
}

func TestMuEdServeHealth_RuntimeError(t *testing.T) {
	mockRuntime := new(MockRuntime)
	mockRuntime.On("Handle", mock.Anything, mock.Anything).
		Return(runtime.EvaluationResponse{}, errors.New("worker unavailable"))

	req := httptest.NewRequest(http.MethodGet, "/evaluate/health", nil)
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeHealth(w, req)

	res := w.Result()
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	assert.Equal(t, "0.1.0", res.Header.Get("X-Api-Version"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	assert.Equal(t, "INTERNAL_ERROR", body["code"])

	mockRuntime.AssertExpectations(t)
}

func TestMuEdServeHealth_DegradedStatus(t *testing.T) {
	healthResult := map[string]any{"tests_passed": false, "successes": []any{}, "failures": []any{"f1"}, "errors": []any{}}
	mockRuntime := new(MockRuntime)
	mockRuntime.On("Handle", mock.Anything, runtime.EvaluationRequest{
		Command: runtime.CommandEvaluateHealth,
		Data:    map[string]any{},
	}).Return(runtime.EvaluationResponse{
		"command": "healthcheck",
		"result":  healthResult,
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/evaluate/health", nil)
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeHealth(w, req)

	res := w.Result()
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "0.1.0", res.Header.Get("X-Api-Version"))

	var result map[string]any
	require.NoError(t, json.Unmarshal(raw, &result))
	assert.Equal(t, "DEGRADED", result["status"])

	mockRuntime.AssertExpectations(t)
}

// --- Version header tests (ServeEvaluate) ---

func TestMuEdServeEvaluate_AbsentVersionHeader(t *testing.T) {
	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).
		Return(evalHandlerResponse(true, "ok"))

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(mathEvalBody(t)))
	w := httptest.NewRecorder()

	newMuEdHandler(mockHandler, nil, "").ServeEvaluate(w, req)

	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "0.1.0", res.Header.Get("X-Api-Version"))
}

func TestMuEdServeEvaluate_SupportedVersionHeader(t *testing.T) {
	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).
		Return(evalHandlerResponse(true, "ok"))

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(mathEvalBody(t)))
	req.Header.Set("X-Api-Version", "0.1.0")
	w := httptest.NewRecorder()

	newMuEdHandler(mockHandler, nil, "").ServeEvaluate(w, req)

	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "0.1.0", res.Header.Get("X-Api-Version"))
}

func TestMuEdServeEvaluate_UnsupportedVersionHeader(t *testing.T) {
	mockHandler := new(MockHandler)

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(mathEvalBody(t)))
	req.Header.Set("X-Api-Version", "99.0.0")
	w := httptest.NewRecorder()

	newMuEdHandler(mockHandler, nil, "").ServeEvaluate(w, req)

	res := w.Result()
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)

	assert.Equal(t, http.StatusNotAcceptable, res.StatusCode)
	assert.Equal(t, "0.1.1-dev", res.Header.Get("X-Api-Version"), "406 stamps the latest supported version")

	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	assert.Equal(t, "VERSION_NOT_SUPPORTED", body["code"])
	details, ok := body["details"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "99.0.0", details["requestedVersion"])

	mockHandler.AssertNotCalled(t, "Handle", mock.Anything, mock.Anything)
}

// --- Version header tests (ServeHealth) ---

func TestMuEdServeHealth_AbsentVersionHeader(t *testing.T) {
	healthResult := map[string]any{"tests_passed": true, "successes": []any{}, "failures": []any{}, "errors": []any{}}
	mockRuntime := new(MockRuntime)
	mockRuntime.On("Handle", mock.Anything, runtime.EvaluationRequest{
		Command: runtime.CommandEvaluateHealth,
		Data:    map[string]any{},
	}).Return(runtime.EvaluationResponse{
		"command": "healthcheck",
		"result":  healthResult,
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/evaluate/health", nil)
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeHealth(w, req)

	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "0.1.0", res.Header.Get("X-Api-Version"))
	mockRuntime.AssertExpectations(t)
}

func TestMuEdServeHealth_UnsupportedVersionHeader(t *testing.T) {
	mockRuntime := new(MockRuntime)

	req := httptest.NewRequest(http.MethodGet, "/evaluate/health", nil)
	req.Header.Set("X-Api-Version", "99.0.0")
	w := httptest.NewRecorder()

	newMuEdHandler(nil, mockRuntime, "").ServeHealth(w, req)

	res := w.Result()
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)

	assert.Equal(t, http.StatusNotAcceptable, res.StatusCode)
	assert.Equal(t, "0.1.1-dev", res.Header.Get("X-Api-Version"), "406 stamps the latest supported version")

	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	assert.Equal(t, "VERSION_NOT_SUPPORTED", body["code"])

	mockRuntime.AssertNotCalled(t, "Handle", mock.Anything, mock.Anything)
}

// --- Request ID tests (ServeEvaluate) ---

func TestMuEdServeEvaluate_RequestID_EchoedWhenSupplied(t *testing.T) {
	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).
		Return(evalHandlerResponse(true, "ok"))

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(mathEvalBody(t)))
	req.Header.Set(muEdRequestIDHeader, "caller-supplied-id")
	w := httptest.NewRecorder()

	newMuEdHandler(mockHandler, nil, "").ServeEvaluate(w, req)

	assert.Equal(t, "caller-supplied-id", w.Result().Header.Get(muEdRequestIDHeader))
}

func TestMuEdServeEvaluate_RequestID_GeneratedWhenAbsent(t *testing.T) {
	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).
		Return(evalHandlerResponse(true, "ok"))

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(mathEvalBody(t)))
	w := httptest.NewRecorder()

	newMuEdHandler(mockHandler, nil, "").ServeEvaluate(w, req)

	assert.NotEmpty(t, w.Result().Header.Get(muEdRequestIDHeader))
}

func TestMuEdServeEvaluate_RequestID_EchoedOnErrorResponses(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader([]byte("not json")))
	req.Header.Set(muEdRequestIDHeader, "caller-supplied-id")
	w := httptest.NewRecorder()

	newMuEdHandler(new(MockHandler), nil, "").ServeEvaluate(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
	assert.Equal(t, "caller-supplied-id", w.Result().Header.Get(muEdRequestIDHeader))
}

func TestMuEdServeEvaluate_ProgressCallback_GeneratedRequestIDUsedAsCorrelation(t *testing.T) {
	srv, received := newProgressCallbackServer(t, nil)

	mockHandler := new(MockHandler)
	mockHandler.On("Handle", mock.Anything, mock.Anything).
		Return(evalHandlerResponse(true, "Well done"))

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(mathEvalBodyWithCallback(t, srv.URL)))
	w := httptest.NewRecorder()

	newMuEdHandlerWithProgress(mockHandler, nil, "", newProgressFactory(t, time.Second)).ServeEvaluate(w, req)

	respRequestID := w.Result().Header.Get(muEdRequestIDHeader)
	require.NotEmpty(t, respRequestID)

	require.Len(t, *received, 1)
	assert.Equal(t, respRequestID, (*received)[0]["correlationId"])
}
