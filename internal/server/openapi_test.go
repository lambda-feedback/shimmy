package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLoadOpenAPISpec(t *testing.T) {
	spec, err := LoadOpenAPISpec()
	require.NoError(t, err)
	assert.NotNil(t, spec)
}

func TestOpenAPIMiddleware_Init(t *testing.T) {
	spec, err := LoadOpenAPISpec()
	require.NoError(t, err)

	middleware, err := OpenAPIMiddleware(spec, zap.NewNop())
	require.NoError(t, err)
	assert.NotNil(t, middleware)
}

func TestOpenAPIMiddleware_UnknownRoute_PassesThrough(t *testing.T) {
	middleware := mustMiddleware(t)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/not-a-mued-route", nil)
	w := httptest.NewRecorder()
	middleware(next).ServeHTTP(w, req)

	assert.True(t, called, "next handler should be called for unknown route")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestOpenAPIMiddleware_ValidRequest_ReachesHandler(t *testing.T) {
	middleware := mustMiddleware(t)

	body := mustJSON(t, map[string]any{
		"submission": map[string]any{
			"type":    "TEXT",
			"content": map[string]any{"text": "hello"},
		},
	})

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`)) //nolint:errcheck
	})

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	middleware(next).ServeHTTP(w, req)

	assert.True(t, called, "next handler should be called for valid request")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestOpenAPIMiddleware_MissingRequiredField_Returns400(t *testing.T) {
	middleware := mustMiddleware(t)

	// POST /evaluate requires "submission"
	body := mustJSON(t, map[string]any{})

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler must not be called for invalid request")
	})

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	middleware(next).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOpenAPIMiddleware_InvalidResponseBody_Returns500(t *testing.T) {
	middleware := mustMiddleware(t)

	body := mustJSON(t, map[string]any{
		"submission": map[string]any{
			"type":    "TEXT",
			"content": map[string]any{"text": "hello"},
		},
	})

	// handler returns an object, but spec requires an array for POST /evaluate 200
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"unexpected": "object"}`)) //nolint:errcheck
	})

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	middleware(next).ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestOpenAPIMiddleware_ValidHealthRequest_ReachesHandler(t *testing.T) {
	middleware := mustMiddleware(t)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(mustJSON(t, map[string]any{ //nolint:errcheck
			"status": "OK",
			"capabilities": map[string]any{
				"supportsEvaluate":              true,
				"supportsPreSubmissionFeedback": false,
				"supportsFormativeFeedback":     true,
				"supportsSummativeFeedback":     true,
				"supportsDataPolicy":            "NOT_SUPPORTED",
			},
		}))
	})

	req := httptest.NewRequest(http.MethodGet, "/evaluate/health", nil)
	w := httptest.NewRecorder()
	middleware(next).ServeHTTP(w, req)

	assert.True(t, called, "next handler should be called for valid health request")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestOpenAPIMiddleware_SSEEvaluate_BypassesResponseValidation(t *testing.T) {
	middleware := mustMiddleware(t)

	body := mustJSON(t, map[string]any{
		"submission": map[string]any{
			"type":    "TEXT",
			"content": map[string]any{"text": "hello"},
		},
	})

	var flushed bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A non-JSON, non-spec body that the buffered path would 500.
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("event: completed\ndata: {}\n\n")) //nolint:errcheck
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
			flushed = true
		}
	})

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()
	middleware(next).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "event: completed\ndata: {}\n\n", w.Body.String())
	assert.True(t, flushed, "handler should receive a flushable writer")
}

func TestOpenAPIMiddleware_SSEChat_BypassesResponseValidation(t *testing.T) {
	middleware := mustMiddleware(t)

	body := mustJSON(t, map[string]any{
		"messages": []map[string]any{{"role": "USER", "content": "hello"}},
	})

	var flushed bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("event: completed\ndata: {}\n\n")) //nolint:errcheck
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
			flushed = true
		}
	})

	req := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()
	middleware(next).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "event: completed\ndata: {}\n\n", w.Body.String())
	assert.True(t, flushed, "handler should receive a flushable writer")
}

func TestOpenAPIMiddleware_SSEEvaluate_RequestStillValidated(t *testing.T) {
	middleware := mustMiddleware(t)

	// missing required "submission"
	body := mustJSON(t, map[string]any{})

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler must not be called for invalid request")
	})

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()
	middleware(next).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// A request that asks for an SSE stream but whose handler falls back to a
// buffered JSON response (e.g. streaming not supported in this runtime)
// must still be response-validated — the bypass keys on what the handler
// wrote, not on the request's Accept header.
func TestOpenAPIMiddleware_AcceptSSEButJSONResponse_StillValidated(t *testing.T) {
	middleware := mustMiddleware(t)

	body := mustJSON(t, map[string]any{
		"submission": map[string]any{
			"type":    "TEXT",
			"content": map[string]any{"text": "hello"},
		},
	})

	// object body: valid JSON but spec requires an array for POST /evaluate 200
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"unexpected": "object"}`)) //nolint:errcheck
	})

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()
	middleware(next).ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// A streamed response is forwarded verbatim and left unvalidated even
// when its body would fail the spec, and the handler still gets a
// flushable writer.
func TestOpenAPIMiddleware_StreamedResponse_ForwardedUnvalidated(t *testing.T) {
	middleware := mustMiddleware(t)

	body := mustJSON(t, map[string]any{
		"submission": map[string]any{
			"type":    "TEXT",
			"content": map[string]any{"text": "hello"},
		},
	})

	var flushed bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// no explicit WriteHeader — the sniffer must decide on first Write
		w.Write([]byte("event: completed\ndata: {\"not\":\"an array\"}\n\n")) //nolint:errcheck
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
			flushed = true
		}
	})

	req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	middleware(next).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "event: completed\ndata: {\"not\":\"an array\"}\n\n", w.Body.String())
	assert.True(t, flushed, "handler should receive a flushable writer")
}

// mustMiddleware loads the real spec and returns the initialised middleware, failing the test on error.
func mustMiddleware(t *testing.T) func(http.Handler) http.Handler {
	t.Helper()
	spec, err := LoadOpenAPISpec()
	require.NoError(t, err)
	middleware, err := OpenAPIMiddleware(spec, zap.NewNop())
	require.NoError(t, err)
	return middleware
}

// mustJSON marshals v to JSON, failing the test on error.
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}
