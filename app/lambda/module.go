package lambda

import (
	"github.com/lambda-feedback/shimmy/handler"
	"github.com/lambda-feedback/shimmy/util/logging"
	"go.uber.org/fx"
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
		// provide server
		fx.Provide(NewLifecycleHandler),
		// invoke server
		fx.Invoke(func(*LambdaHandler) {}),
	)
}
