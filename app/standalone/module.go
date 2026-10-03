package standalone

import (
	"go.uber.org/fx"

	"github.com/lambda-feedback/shimmy/handler"
	"github.com/lambda-feedback/shimmy/internal/server"
	"github.com/lambda-feedback/shimmy/util/logging"
)

func Module(config Config) fx.Option {
	return fx.Module(
		"serve",
		// rename logger for module
		logging.DecorateLogger("serve"),
		// the standalone HTTP server can stream responses incrementally
		fx.Supply(handler.StreamingCapability{Enabled: true}),
		// provide handlers
		handler.Module(),
		// provide server
		server.Module(config.HttpConfig),
	)
}
