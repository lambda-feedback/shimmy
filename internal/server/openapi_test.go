package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLoadOpenAPISpec(t *testing.T) {
	spec, err := LoadOpenAPISpec()
	require.NoError(t, err)
	assert.NotNil(t, spec)
}

func TestLoadOpenAPISpecs(t *testing.T) {
	specs, err := LoadOpenAPISpecs()
	require.NoError(t, err)
	require.NotEmpty(t, specs)
	assert.Contains(t, specs, "0.1.0")
	assert.Contains(t, specs, "0.1.1-dev")
	for version, spec := range specs {
		assert.NotNilf(t, spec, "spec for %s", version)
	}

	// The opt-in SSE progress-streaming surface is a shimmy extension pending
	// upstream µEd PRs: it lives in 0.1.1-dev only, never in canonical 0.1.0.
	sseSchemas := []string{
		"SseProgressStep", "SseTerminalSteps", "StreamingCapabilities",
		"SseChatTerminalFrame", "SseEvaluateTerminalFrame",
	}
	for _, name := range sseSchemas {
		assert.Containsf(t, specs["0.1.1-dev"].Components.Schemas, name, "0.1.1-dev should define %s", name)
		assert.NotContainsf(t, specs["0.1.0"].Components.Schemas, name, "canonical 0.1.0 must not define %s", name)
	}
}

func TestOpenAPIMiddleware_Init(t *testing.T) {
	specs, err := LoadOpenAPISpecs()
	require.NoError(t, err)

	middleware, err := OpenAPIMiddleware(specs, nil, zap.NewNop())
	require.NoError(t, err)
	assert.NotNil(t, middleware)
}

func TestOpenAPIMiddleware_NoSpecs_Errors(t *testing.T) {
	_, err := OpenAPIMiddleware(map[string]*openapi3.T{}, nil, zap.NewNop())
	assert.Error(t, err)
}

func TestOpenAPIMiddleware_UnknownRoute_PassesThrough(t *testing.T) {
	middleware := mustMiddleware(t)

	for _, version := range []string{"", "0.1.0", "0.2.0", "9.9.9"} {
		called := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/not-a-mued-route", nil)
		if version != "" {
			req.Header.Set("X-Api-Version", version)
		}
		w := httptest.NewRecorder()
		middleware(next).ServeHTTP(w, req)

		assert.Truef(t, called, "next handler should be called for unknown route (version %q)", version)
		assert.Equal(t, http.StatusOK, w.Code)
	}
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

// TestOpenAPIMiddleware_VersionSelectsSpec proves the middleware validates a
// request against the spec for the version the client targets: the same
// /evaluate body is accepted under v0.1.0 but rejected under the synthetic
// v0.2.0 spec, which additionally requires "extraField".
func TestOpenAPIMiddleware_VersionSelectsSpec(t *testing.T) {
	bodyNoExtra := map[string]any{
		"submission": map[string]any{"type": "TEXT", "content": map[string]any{"text": "hi"}},
	}
	bodyWithExtra := map[string]any{
		"submission": map[string]any{"type": "TEXT", "content": map[string]any{"text": "hi"}},
		"extraField": "present",
	}

	tests := []struct {
		name        string
		version     string
		body        map[string]any
		wantCode    int
		wantHandler bool
	}{
		{"v0.1.0 accepts body without extraField", "0.1.0", bodyNoExtra, http.StatusOK, true},
		{"no header resolves to v0.1.0 and accepts", "", bodyNoExtra, http.StatusOK, true},
		{"v0.2.0 rejects body without extraField", "0.2.0", bodyNoExtra, http.StatusBadRequest, false},
		{"v0.2.0 accepts body with extraField", "0.2.0", bodyWithExtra, http.StatusOK, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			middleware := mustMiddleware(t)

			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`[]`)) //nolint:errcheck
			})

			req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader(mustJSON(t, tc.body)))
			req.Header.Set("Content-Type", "application/json")
			if tc.version != "" {
				req.Header.Set("X-Api-Version", tc.version)
			}
			w := httptest.NewRecorder()
			middleware(next).ServeHTTP(w, req)

			assert.Equal(t, tc.wantCode, w.Code)
			assert.Equal(t, tc.wantHandler, called)
		})
	}
}

// mustMiddleware loads the real v0.1.0 spec plus the synthetic v0.2.0 testdata
// spec and returns the initialised middleware, with a resolver that mirrors
// runtime.MuEdRegistry.Resolve for the order [0.1.0, 0.2.0].
func mustMiddleware(t *testing.T) func(http.Handler) http.Handler {
	t.Helper()

	specs, err := LoadOpenAPISpecs()
	require.NoError(t, err)

	data, err := os.ReadFile("testdata/mued_v0.2.0.yml")
	require.NoError(t, err)
	v020, err := loadSpec(data)
	require.NoError(t, err)
	specs["0.2.0"] = v020

	resolve := func(v string) string {
		switch v {
		case "":
			return "0.1.0" // Default(): first registered, pinned
		case "0.1.0", "0.2.0":
			return v
		default:
			return "0.2.0" // Latest()
		}
	}

	middleware, err := OpenAPIMiddleware(specs, resolve, zap.NewNop())
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
