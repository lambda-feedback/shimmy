package standalone

import (
	"github.com/lambda-feedback/shimmy/handler/common"
	"github.com/lambda-feedback/shimmy/internal/server"
	"github.com/lambda-feedback/shimmy/util/logging"
	"go.uber.org/fx"
)

func Module(config server.HttpConfig) fx.Option {
	return fx.Module(
		"serve",
		// rename logger for module
		logging.DecorateLogger("serve"),
		// provide handlers
		common.Module(),
		// provide server
		server.Module(config),
	)
}
