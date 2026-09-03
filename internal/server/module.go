package server

import "go.uber.org/fx"

// HandlerModule provides the shared application HTTP handler chain — the
// per-version OpenAPI specs and the wrapped Mux. Both the standalone server and
// the Lambda adapter depend on it so they serve an identical, identically
// validated handler.
func HandlerModule() fx.Option {
	return fx.Module("http-handler",
		// provide openapi specs (one per supported µEd version)
		fx.Provide(LoadOpenAPISpecs),
		// provide the wrapped handler chain
		fx.Provide(NewMux),
	)
}

// Module provides the standalone HTTP server on top of HandlerModule.
func Module(config HttpConfig) fx.Option {
	return fx.Module("server",
		// provide config
		fx.Supply(config),
		// provide the shared handler chain
		HandlerModule(),
		// provide server
		fx.Provide(NewLifecycleServer),
		// invoke server
		fx.Invoke(func(*HttpServer) {}),
	)
}
