package standalone

import (
	"context"
	"testing"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/lambda-feedback/shimmy/config"
	"github.com/lambda-feedback/shimmy/runtime"
	"github.com/stretchr/testify/assert"
)

// TestModule_DependencyGraphValid makes sure the standalone fx graph — the µEd
// handlers, the per-version OpenAPI specs, and the µEd version registry provided
// by runtime.Module — resolves without missing or cyclic dependencies.
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
