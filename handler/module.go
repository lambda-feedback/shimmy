package handler

import (
	"go.uber.org/fx"

	"github.com/lambda-feedback/shimmy/config"
	"github.com/lambda-feedback/shimmy/internal/progress"
)

// StreamingCapability tells the muEd handler whether the current
// execution environment can stream an HTTP response incrementally.
// It is true under the standalone HTTP server and false under the AWS
// Lambda proxy (which buffers the whole response). Each app module
// supplies its own value — it is deliberately not provided here.
type StreamingCapability struct {
	Enabled bool
}

func Module() fx.Option {
	return fx.Module("common",
		fx.Provide(NewCommandHandler),
		fx.Provide(NewMuEdHandler),
		fx.Provide(NewLegacyRoute),
		fx.Provide(NewHealthRoute),
		fx.Provide(NewMuEdEvaluateRoute),
		fx.Provide(NewMuEdEvaluateHealthRoute),
		fx.Provide(func(cfg config.Config) progress.Config { return cfg.Progress }),
		fx.Provide(progress.NewHTTPFactory),
		fx.Provide(NewMuEdChatRoute),
		fx.Provide(NewMuEdChatHealthRoute),
	)
}
