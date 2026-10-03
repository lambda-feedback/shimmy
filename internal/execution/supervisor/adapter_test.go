package supervisor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/lambda-feedback/shimmy/internal/execution/worker"
	"github.com/lambda-feedback/shimmy/internal/progress"
)

func TestDefaultAdapterFactory(t *testing.T) {
	w := worker.NewMockWorker(t)

	workerFactory := func(worker.StartConfig) (worker.Worker, error) {
		return w, nil
	}

	factory := newDefaultAdapterFactory(progress.Config{})

	cases := []IOConfig{{Interface: FileIO}, {Interface: RpcIO}}
	for _, mode := range cases {
		_, err := factory(workerFactory, mode, zap.NewNop())

		assert.NoError(t, err)
	}
}

func TestDefaultAdapterFactory_Fails(t *testing.T) {
	w := worker.NewMockWorker(t)

	workerFactory := func(worker.StartConfig) (worker.Worker, error) {
		return w, nil
	}

	_, err := newDefaultAdapterFactory(progress.Config{})(workerFactory, IOConfig{Interface: ""}, zap.NewNop())

	assert.ErrorIs(t, err, ErrUnsupportedIOInterface)
}
