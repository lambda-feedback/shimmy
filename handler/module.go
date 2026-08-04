package handler

import (
	"go.uber.org/fx"

	"github.com/lambda-feedback/shimmy/config"
	"github.com/lambda-feedback/shimmy/internal/progress"
)

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
	)
}
