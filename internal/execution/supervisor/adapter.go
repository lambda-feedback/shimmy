package supervisor

import (
	"context"

	"github.com/lambda-feedback/shimmy/internal/execution/models"
	"github.com/lambda-feedback/shimmy/internal/execution/worker"
	"go.uber.org/zap"
)

type AdapterFactoryFn[I, M, O any] func(IOInterface, *zap.Logger) (Adapter[I, M, O], error)

type WaitFunc func() error

type Adapter[I, M, O any] interface {
	Start(context.Context, worker.StartConfig) error
	Stop(context.Context, worker.StopConfig) (WaitFunc, error)
	Send(context.Context, models.Message[I, M], worker.SendConfig) (O, error)
}

// MARK: - factory

func defaultAdapterFactory[I, M, O any](
	mode IOInterface,
	log *zap.Logger,
) (Adapter[I, M, O], error) {
	switch mode {
	case FileIO:
		return newFileAdapter[I, M, O](log), nil
	case StdIO:
		return newStdioAdapter[I, M, O](log), nil
	// case SocketIO:
	// 	return &socketAdapter[I, O]{log: log}, nil
	default:
		return nil, ErrUnsupportedIOMode
	}
}

// MARK: - helpers

func stopWorker[I, M, O any](
	ctx context.Context,
	w worker.Worker[I, M, O],
	params worker.StopConfig,
) (WaitFunc, error) {

	// TODO: what if shutdown fails? we have a zombie worker then...

	// gracefully shutdown the worker
	if err := w.Terminate(); err != nil {
		// no need to wait for termination if we could not terminate
		return nil, err
	}

	waitFunc := func() error {
		// wait for the worker to terminate
		_, err := w.WaitFor(ctx, params.Timeout)
		if err != nil {
			return err
		}

		return nil
	}

	return waitFunc, nil
}
