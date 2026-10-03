package server

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"
	"go.uber.org/zap"

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
// nil resolveVersion falls back to defaultSpecVersionResolver, which routes off
// the loaded spec versions alone (this package can't import runtime without an
// import cycle via internal/progress). Routes that no selected spec defines
// (e.g. the legacy "/" route) pass through unvalidated.
func OpenAPIMiddleware(specs map[string]*openapi3.T, resolveVersion func(string) string, log *zap.Logger) (func(http.Handler) http.Handler, error) {
	if len(specs) == 0 {
		return nil, fmt.Errorf("no OpenAPI specs provided")
	}
	if resolveVersion == nil {
		resolveVersion = defaultSpecVersionResolver(specs)
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

			// Buffer the response so it can be validated — unless the
			// handler streams it (Content-Type: text/event-stream), in
			// which case the sniffer has already committed it to the
			// client and there is nothing to validate: the filter has no
			// model for a frame sequence and buffering would defeat the
			// stream. The decision follows what the handler actually did,
			// so it can't disagree with the handler's own streaming check.
			sniffer := newResponseSniffer(w)
			next.ServeHTTP(sniffer, r)
			if sniffer.streamed() {
				return
			}

			bodyBytes := sniffer.buf.Bytes()

			// Validate response
			respInput := &openapi3filter.ResponseValidationInput{
				RequestValidationInput: reqInput,
				Status:                 sniffer.status,
				Header:                 sniffer.Header(),
				Body:                   io.NopCloser(bytes.NewReader(bodyBytes)),
				Options:                opts,
			}
			if err := openapi3filter.ValidateResponse(r.Context(), respInput); err != nil {
				log.Error("response failed OpenAPI validation", zap.Error(err))
				http.Error(w, "invalid response format", http.StatusInternalServerError)
				return
			}

			// Forward the buffered response. Headers set by the handler are
			// already on w — the sniffer passed w's header map through.
			w.WriteHeader(sniffer.status)
			w.Write(bodyBytes) //nolint:errcheck
		})
	}, nil
}

// defaultSpecVersionResolver resolves an X-Api-Version header against the set
// of loaded spec versions when the caller passes no resolver: an exact match
// wins, an empty header selects the lowest version, and anything else selects
// the highest. With a single embedded spec (the common case) every input
// resolves to that one version. It mirrors runtime.MuEdRegistry.Resolve
// closely enough for validation routing, without importing runtime.
func defaultSpecVersionResolver(specs map[string]*openapi3.T) func(string) string {
	versions := make([]string, 0, len(specs))
	for v := range specs {
		versions = append(versions, v)
	}
	sort.Strings(versions)

	return func(requested string) string {
		if _, ok := specs[requested]; ok {
			return requested
		}
		if len(versions) == 0 {
			return requested
		}
		if requested == "" {
			return versions[0]
		}
		return versions[len(versions)-1]
	}
}
