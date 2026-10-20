package server

import (
	"fmt"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// MuxParams are the dependency-injected pieces the shared HTTP handler chain is
// built from.
type MuxParams struct {
	fx.In

	Specs    map[string]*openapi3.T
	Handlers []*HttpHandler `group:"handlers"`
	Logger   *zap.Logger
}

// Mux is the fully-wrapped application HTTP handler: the route mux, path
// normalisation, and per-version OpenAPI request/response validation. Both the
// standalone server and the Lambda adapter serve this exact chain, so the two
// deployments validate requests and responses identically.
type Mux struct {
	http.Handler
}

// NewMux assembles the shared HTTP handler chain from the registered route
// handlers and the embedded OpenAPI specs.
func NewMux(params MuxParams) (*Mux, error) {
	mux := http.NewServeMux()
	for _, h := range params.Handlers {
		mux.Handle(h.Name, h.Handler)
	}

	openAPIMiddleware, err := OpenAPIMiddleware(params.Specs, nil, params.Logger)
	if err != nil {
		return nil, fmt.Errorf("initialising OpenAPI middleware: %w", err)
	}

	return &Mux{Handler: openAPIMiddleware(NormalizePath(mux))}, nil
}
