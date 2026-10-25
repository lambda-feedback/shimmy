package lambda

import (
	"go.uber.org/fx"

	"github.com/lambda-feedback/shimmy/handler"
	"github.com/lambda-feedback/shimmy/internal/server"
	"github.com/lambda-feedback/shimmy/util/logging"
)

func Module(config Config) fx.Option {
	return fx.Module(
		"lambda",
		// provide lambda config
		fx.Supply(config),
		// rename logger for module
		logging.DecorateLogger("lambda"),
		// provide handlers
		handler.Module(),
		// provide the shared HTTP handler chain (specs + wrapped mux)
		server.HandlerModule(),
		// provide server
		fx.Provide(NewLifecycleHandler),
		// invoke server
		fx.Invoke(func(*LambdaHandler) {}),
	)
}
