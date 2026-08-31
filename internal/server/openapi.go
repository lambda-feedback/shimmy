package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"
	"go.uber.org/zap"

	"github.com/lambda-feedback/shimmy/runtime/schema"
)

func LoadOpenAPISpec() (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	spec, err := loader.LoadFromData(schema.OpenAPISpec)
	if err != nil {
		return nil, fmt.Errorf("loading OpenAPI spec: %w", err)
	}
	// Skip validation for OpenAPI 3.1.0 — the legacy router validates on NewRouter.
	return spec, nil
}

// sseStreamsProgressRoute reports whether this request is an SSE-streaming
// POST to /evaluate or /chat: its response is written and flushed
// incrementally, so the middleware must not buffer it through
// httptest.NewRecorder (which also strips http.Flusher) or validate its
// non-JSON body against the spec. Request validation still runs. Matched
// with HasSuffix because the middleware runs before NormalizePath rewrites
// the path; "/chat/health" (GET) does not end with "/chat" and so is never
// matched.
func sseStreamsProgressRoute(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	if !strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream") {
		return false
	}
	return strings.HasSuffix(r.URL.Path, "/evaluate") || strings.HasSuffix(r.URL.Path, "/chat")
}

func OpenAPIMiddleware(spec *openapi3.T, log *zap.Logger, sseEnabled bool) (func(http.Handler) http.Handler, error) {
	router, err := legacy.NewRouter(spec,
		openapi3.IsOpenAPI31OrLater(),
		openapi3.AllowExtraSiblingFields("description", "summary"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating OpenAPI router: %w", err)
	}
	opts := &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route, pathParams, err := router.FindRoute(r)
			if err != nil {
				// Not a µEd route — pass through unvalidated
				next.ServeHTTP(w, r)
				return
			}

			// Validate request
			reqInput := &openapi3filter.RequestValidationInput{
				Request:    r,
				PathParams: pathParams,
				Route:      route,
				Options:    opts,
			}
			if err := openapi3filter.ValidateRequest(r.Context(), reqInput); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			// A streaming SSE response can't be buffered or JSON-validated;
			// hand the real writer straight to the handler.
			if sseEnabled && sseStreamsProgressRoute(r) {
				next.ServeHTTP(w, r)
				return
			}

			// Capture response for validation
			rec := httptest.NewRecorder()
			next.ServeHTTP(rec, r)

			// Snapshot body before validation — ValidateResponse drains the buffer.
			bodyBytes := rec.Body.Bytes()

			// Validate response (lenient — log only)
			respInput := &openapi3filter.ResponseValidationInput{
				RequestValidationInput: reqInput,
				Status:                 rec.Code,
				Header:                 rec.Header(),
				Body:                   io.NopCloser(bytes.NewReader(bodyBytes)),
				Options:                opts,
			}
			if err := openapi3filter.ValidateResponse(r.Context(), respInput); err != nil {
				log.Error("response failed OpenAPI validation", zap.Error(err))
				http.Error(w, "invalid response format", http.StatusInternalServerError)
				return
			}

			// Forward captured response
			for k, v := range rec.Header() {
				w.Header()[k] = v
			}
			w.WriteHeader(rec.Code)
			w.Write(bodyBytes) //nolint:errcheck
		})
	}, nil
}
