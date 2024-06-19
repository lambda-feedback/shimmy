package dispatcher

import (
	"context"
	"fmt"

	"github.com/jackc/puddle/v2"
	"github.com/lambda-feedback/shimmy/internal/execution/supervisor"
	"go.uber.org/zap"
)

type PooledDispatcher struct {
	ctx  context.Context
	pool *puddle.Pool[supervisor.Supervisor]
	log  *zap.Logger
}

var _ Dispatcher = (*PooledDispatcher)(nil)

type PooledDispatcherConfig struct {
	// MaxWorkers is the maximum number of concurrent workers
	MaxWorkers int `conf:"max_workers"`

	// SupervisorConfig is the configuration to use for the supervisor
	Supervisor supervisor.Config `conf:"supervisor,squash"`
}

type PooledDispatcherParams struct {
	// Context is the context to use for the dispatcher
	Context context.Context

	// Config is the config for the dispatcher and the underlying supervisors
	Config PooledDispatcherConfig

	// SupervisorFactory is the factory function to create a new supervisor
	SupervisorFactory SupervisorFactory

	// Log is the logger to use for the dispatcher
	Log *zap.Logger
}

func NewPooledDispatcher(
	params PooledDispatcherParams,
) (Dispatcher, error) {
	if params.SupervisorFactory == nil {
		params.SupervisorFactory = defaultSupervisorFactory
	}

	pool, err := createPool(params)
	if err != nil {
		return nil, err
	}

	return &PooledDispatcher{
		pool: pool,
		ctx:  params.Context,
		log:  params.Log.Named("dispatcher_pooled"),
	}, nil
}

func (m *PooledDispatcher) Start(context.Context) error {
	// starting the pool is a no-op
	return nil
}

func (m *PooledDispatcher) Send(ctx context.Context, result any, method string, data map[string]any) error {

	resource, err := m.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("error acquiring supervisor: %w", err)
	}

	err = m.sendToSupervisor(ctx, result, method, data, resource)
	if err != nil {
		return fmt.Errorf("error sending data: %w", err)
	}

	return nil
}

func (m *PooledDispatcher) sendToSupervisor(
	ctx context.Context,
	result any,
	method string,
	data map[string]any,
	resource *puddle.Resource[supervisor.Supervisor],
) error {
	var err error
	var release supervisor.ReleaseFunc

	destroyOrRelease := func() {
		if err != nil {
			m.log.Debug("destroying supervisor due to error")
			resource.Destroy()
		} else {
			m.log.Debug("releasing supervisor back to pool")
			resource.Release()
		}
	}

	dispose := func() {
		// if there is no wait function, we can release the resource
		if release == nil {
			destroyOrRelease()
			return
		}

		// if there is an error destroying the supervisor, we need to
		// log it and destroy the resource
		if releaseErr := release(m.ctx); releaseErr != nil {
			m.log.Error("destroying supervisor due to error waiting", zap.Error(releaseErr))
			resource.Destroy()
			return
		}

		// if the supervisor was suspended, we release the resource
		destroyOrRelease()
	}

	defer func() {
		// we need to release the supervisor back to the pool after
		// we're done with it. however, we want to return from the
		// function before releasing the supervisor, as we don't want
		// to block the caller. therefore, we release the supervisor
		// in a goroutine.
		go dispose()
	}()

	supervisor := resource.Value()

	release, err = supervisor.Send(ctx, result, method, data)
	if err != nil {
		return err
	}

	return nil
}

// Shutdown stops the dispatcher and waits for all workers to finish.
func (m *PooledDispatcher) Shutdown(context.Context) error {
	m.log.Debug("shutting down dispatcher")
	m.pool.Close()
	return nil
}

// MARK: - Pool

func createPool(
	params PooledDispatcherParams,
) (*puddle.Pool[supervisor.Supervisor], error) {
	log := params.Log.Named("dispatcher_pool")

	constructor := func(ctx context.Context) (supervisor.Supervisor, error) {
		sv, err := params.SupervisorFactory(supervisor.Params{
			Config: params.Config.Supervisor,
			Log:    params.Log,
		})
		if err != nil {
			return nil, err
		}

		if err = sv.Start(ctx); err != nil {
			return nil, err
		}

		return sv, nil
	}

	destructor := func(s supervisor.Supervisor) {
		wait, err := s.Shutdown(params.Context)
		if err != nil {
			log.Error("error shutting down supervisor", zap.Error(err))
			return
		}

		if wait == nil {
			log.Warn("supervisor did not return a wait function")
			return
		}

		if err := wait(); err != nil {
			log.Error("error waiting for supervisor to shut down", zap.Error(err))
		}
	}

	return puddle.NewPool(&puddle.Config[supervisor.Supervisor]{
		Constructor: constructor,
		Destructor:  destructor,
		MaxSize:     int32(params.Config.MaxWorkers),
	})
}
