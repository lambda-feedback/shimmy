package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
	"go.uber.org/zap"

	"github.com/lambda-feedback/shimmy/runtime"
	"github.com/lambda-feedback/shimmy/runtime/schema"
)

func loadSpec(data []byte) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	spec, err := loader.LoadFromData(data)
	if err != nil {
		return nil, err
	}
	// Skip validation for OpenAPI 3.1.0 — the legacy router validates on NewRouter.
	return spec, nil
}

// LoadOpenAPISpec loads the latest embedded µEd OpenAPI spec.
func LoadOpenAPISpec() (*openapi3.T, error) {
	spec, err := loadSpec(schema.OpenAPISpec)
	if err != nil {
		return nil, fmt.Errorf("loading OpenAPI spec: %w", err)
	}
	return spec, nil
}

// LoadOpenAPISpecs loads every embedded µEd OpenAPI spec, keyed by version.
func LoadOpenAPISpecs() (map[string]*openapi3.T, error) {
	out := make(map[string]*openapi3.T, len(schema.MuEdOpenAPISpecs))
	for version, data := range schema.MuEdOpenAPISpecs {
		spec, err := loadSpec(data)
		if err != nil {
			return nil, fmt.Errorf("loading OpenAPI spec %s: %w", version, err)
		}
		out[version] = spec
	}
	return out, nil
}

// OpenAPIMiddleware validates µEd requests and responses against the OpenAPI
// spec for the version the client is targeting. The spec is selected from the
// X-Api-Version header via resolveVersion — the same resolver the handlers use —
// so a request is validated against exactly the version that will serve it. A
// nil resolveVersion defaults to runtime.MuEdResolveVersion. Routes that no
// selected spec defines (e.g. the legacy "/" route) pass through unvalidated.
func OpenAPIMiddleware(specs map[string]*openapi3.T, resolveVersion func(string) string, log *zap.Logger) (func(http.Handler) http.Handler, error) {
	if len(specs) == 0 {
		return nil, fmt.Errorf("no OpenAPI specs provided")
	}
	if resolveVersion == nil {
		resolveVersion = runtime.MuEdResolveVersion
	}

	routerByVersion := make(map[string]routers.Router, len(specs))
	for version, spec := range specs {
		router, err := legacy.NewRouter(spec,
			openapi3.IsOpenAPI31OrLater(),
			openapi3.AllowExtraSiblingFields("description", "summary"),
		)
		if err != nil {
			return nil, fmt.Errorf("creating OpenAPI router for %s: %w", version, err)
		}
		routerByVersion[version] = router
	}
	opts := &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			version := resolveVersion(r.Header.Get("X-Api-Version"))
			router, ok := routerByVersion[version]
			if !ok {
				// No spec for the resolved version — cannot validate, pass through.
				// The handler still rejects genuinely unsupported versions with a 406.
				next.ServeHTTP(w, r)
				return
			}

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

			// Capture response for validation
			rec := httptest.NewRecorder()
			next.ServeHTTP(rec, r)

			// Snapshot body before validation — ValidateResponse drains the buffer.
			bodyBytes := rec.Body.Bytes()

			// Validate response
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
