package lambda

import (
	"context"
	"testing"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/lambda-feedback/shimmy/config"
	"github.com/lambda-feedback/shimmy/runtime"
)

// TestModule_DependencyGraphResolves guards the fx wiring for Lambda mode
// given the globals app.New supplies. StreamingCapability is supplied
// here as {Enabled: false} — the Lambda proxy cannot stream. It now also
// pulls in server.HandlerModule so the Lambda adapter serves the same
// OpenAPI-validated handler chain as the standalone server.
func TestModule_DependencyGraphResolves(t *testing.T) {
	cfg := config.Config{}

	err := fx.ValidateApp(
		fx.NopLogger,
		fx.Supply(fx.Annotate(context.Background(), fx.As(new(context.Context)))),
		fx.Supply(zap.NewNop()),
		fx.Supply(cfg),
		runtime.Module(cfg.Runtime),
		Module(Config{}),
	)
	if err != nil {
		t.Fatalf("lambda fx graph failed validation: %v", err)
	}
}
