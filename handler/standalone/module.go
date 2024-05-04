package standalone

import (
	"github.com/lambda-feedback/shimmy/handler/common"
	"github.com/lambda-feedback/shimmy/util/logging"
	"go.uber.org/fx"
)

func Module(config HttpConfig) fx.Option {
	return fx.Module(
		"serve",
		// rename logger for module
		logging.DecorateLogger("serve"),
		// provide config
		fx.Supply(config),
		// provide handlers
		fx.Provide(common.NewCommandHandler),
		fx.Provide(NewLegacyRoute),
		fx.Provide(NewCommandRoute),
		// provide server
		fx.Provide(NewLifecycleServer),
		// invoke server
		fx.Invoke(func(*HttpServer) {}),
	)
}
