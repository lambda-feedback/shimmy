package lambda

import (
	"context"
	"testing"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/lambda-feedback/shimmy/config"
	"github.com/lambda-feedback/shimmy/runtime"
	"github.com/stretchr/testify/assert"
)

// TestModule_DependencyGraphValid makes sure the Lambda fx graph resolves —
// notably that it now pulls in server.HandlerModule so the Lambda adapter serves
// the same OpenAPI-validated handler chain as the standalone server.
func TestModule_DependencyGraphValid(t *testing.T) {
	err := fx.ValidateApp(
		fx.NopLogger,
		fx.Supply(fx.Annotate(context.Background(), fx.As(new(context.Context)))),
		fx.Supply(zap.NewNop()),
		fx.Supply(config.Config{}),
		runtime.Module(config.Config{}.Runtime),
		Module(Config{}),
	)
	assert.NoError(t, err)
}
