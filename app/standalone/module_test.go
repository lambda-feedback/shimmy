package standalone

import (
	"context"
	"testing"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/lambda-feedback/shimmy/config"
	"github.com/lambda-feedback/shimmy/runtime"
)

// TestModule_DependencyGraphResolves guards the fx wiring: the standalone
// module must be satisfiable given the globals app.New supplies (context,
// logger, config.Config, runtime module — which provides the µEd version
// registry). Regressions here — e.g. a handler param with no provider —
// surface as a validation error rather than a runtime panic on
// `shimmy serve`.
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
		t.Fatalf("standalone fx graph failed validation: %v", err)
	}
}
