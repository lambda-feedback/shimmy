package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestNewMux_AppliesValidationAndNormalisation proves the shared chain both
// deployments serve actually wraps the route handlers with OpenAPI validation
// and path normalisation.
func TestNewMux_AppliesValidationAndNormalisation(t *testing.T) {
	specs, err := LoadOpenAPISpecs()
	require.NoError(t, err)

	var gotPath string
	evaluate := &HttpHandler{
		Name: "/evaluate",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[]`)) //nolint:errcheck
		}),
	}

	mux, err := NewMux(MuxParams{
		Specs:    specs,
		Handlers: []*HttpHandler{evaluate},
		Logger:   zap.NewNop(),
	})
	require.NoError(t, err)

	t.Run("valid request is normalised and reaches the handler", func(t *testing.T) {
		gotPath = ""
		body := mustJSON(t, map[string]any{
			"submission": map[string]any{"type": "TEXT", "content": map[string]any{"text": "hi"}},
		})
		req := httptest.NewRequest(http.MethodPost, "/myFunction/evaluate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "/evaluate", gotPath, "NormalizePath should rewrite the prefixed path")
	})

	t.Run("spec-violating request is rejected before the handler", func(t *testing.T) {
		gotPath = ""
		req := httptest.NewRequest(http.MethodPost, "/evaluate", bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Empty(t, gotPath, "handler must not be reached")
	})

	t.Run("unknown route passes through", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/not-a-mued-route", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		// no handler registered for this path -> mux 404, but the middleware
		// must not have turned it into a 400/500
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
