package supervisor

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"github.com/lambda-feedback/shimmy/internal/execution/worker"
	"github.com/lambda-feedback/shimmy/internal/progress"
)

type rwc struct {
	*bytes.Buffer
}

func (rwc *rwc) Close() error {
	return nil
}

func newRwc() io.ReadWriteCloser {
	return &rwc{Buffer: new(bytes.Buffer)}
}

func createRpcAdapter(t *testing.T) (*rpcAdapter, *worker.MockWorker) {
	w := worker.NewMockWorker(t)

	workerFactory := func(worker.StartConfig) (worker.Worker, error) {
		return w, nil
	}

	adapter := &rpcAdapter{
		workerFactory: workerFactory,
		log:           zap.NewNop(),
		config:        RpcConfig{Transport: StdioTransport},
	}

	// Start (called by most tests using this helper) always spins up a
	// real progress sidecar listener; close it so tests don't leak ports.
	t.Cleanup(func() {
		if adapter.sidecar != nil {
			adapter.sidecar.Close()
		}
	})

	return adapter, w
}

func TestStdioAdapter_Start(t *testing.T) {
	a, w := createRpcAdapter(t)

	ctx := context.Background()
	params := worker.StartConfig{}

	w.EXPECT().DuplexPipe().Return(newRwc(), nil)
	w.EXPECT().Start(ctx).Return(nil)

	err := a.Start(ctx, params)
	assert.NoError(t, err)
}

func TestStdioAdapter_Start_PassesError(t *testing.T) {
	a, w := createRpcAdapter(t)

	ctx := context.Background()
	params := worker.StartConfig{}

	w.EXPECT().DuplexPipe().Return(newRwc(), nil)
	w.EXPECT().Start(ctx).Return(assert.AnError)

	err := a.Start(ctx, params)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestStdioAdapter_Stop(t *testing.T) {
	a, w := createRpcAdapter(t)

	w.EXPECT().DuplexPipe().Return(newRwc(), nil)
	w.EXPECT().Start(mock.Anything).Return(nil)
	w.EXPECT().Stop().Return(nil)

	err := a.Start(context.Background(), worker.StartConfig{})
	assert.NoError(t, err)

	_, err = a.Stop()
	assert.NoError(t, err)
}

func TestStdioAdapter_Stop_FailsIfNotStarted(t *testing.T) {
	a, _ := createRpcAdapter(t)

	_, err := a.Stop()
	assert.Error(t, err)
}

func TestStdioAdapter_Stop_PassesError(t *testing.T) {
	a, w := createRpcAdapter(t)

	w.EXPECT().DuplexPipe().Return(newRwc(), nil)
	w.EXPECT().Start(mock.Anything).Return(nil)
	w.EXPECT().Stop().Return(assert.AnError)

	err := a.Start(context.Background(), worker.StartConfig{})
	assert.NoError(t, err)

	_, err = a.Stop()
	assert.ErrorIs(t, err, assert.AnError)
}

func TestStdioAdapter_Stop_WaitFor(t *testing.T) {
	a, w := createRpcAdapter(t)

	ctx := context.Background()

	w.EXPECT().DuplexPipe().Return(newRwc(), nil)
	w.EXPECT().Start(mock.Anything).Return(nil)
	w.EXPECT().Stop().Return(nil)
	w.EXPECT().Wait(ctx).Return(worker.ExitEvent{}, nil)

	err := a.Start(context.Background(), worker.StartConfig{})
	assert.NoError(t, err)

	wait, err := a.Stop()
	assert.NoError(t, err)

	err = wait(ctx)
	assert.NoError(t, err)
}

func TestStdioAdapter_Stop_WaitForError(t *testing.T) {
	a, w := createRpcAdapter(t)

	ctx := context.Background()

	w.EXPECT().DuplexPipe().Return(newRwc(), nil)
	w.EXPECT().Start(mock.Anything).Return(nil)
	w.EXPECT().Stop().Return(nil)
	w.EXPECT().Wait(ctx).Return(worker.ExitEvent{}, assert.AnError)

	err := a.Start(context.Background(), worker.StartConfig{})
	assert.NoError(t, err)

	wait, err := a.Stop()
	assert.NoError(t, err)

	err = wait(ctx)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestStdioAdapter_Start_InjectsProgressURL(t *testing.T) {
	a, w := createRpcAdapter(t)

	var sp *worker.StartConfig
	baseFactory := a.workerFactory
	a.workerFactory = func(params worker.StartConfig) (worker.Worker, error) {
		sp = &params
		return baseFactory(params)
	}

	w.EXPECT().DuplexPipe().Return(newRwc(), nil)
	w.EXPECT().Start(mock.Anything).Return(nil)

	err := a.Start(context.Background(), worker.StartConfig{})
	assert.NoError(t, err)

	assert.Contains(t, sp.Env, "EVAL_PROGRESS_URL="+a.sidecar.URL())
}

// TestStdioAdapter_Send_RelaysWorkerProgressEvents exercises the same
// Bind/UnbindAfterGrace path Send uses around the (separately, more fully)
// tested Sidecar, without needing a live RPC round trip - Send itself isn't
// otherwise exercised in this file (see the disabled tests below).
func TestStdioAdapter_Send_RelaysWorkerProgressEvents(t *testing.T) {
	a, w := createRpcAdapter(t)

	w.EXPECT().DuplexPipe().Return(newRwc(), nil)
	w.EXPECT().Start(mock.Anything).Return(nil)

	err := a.Start(context.Background(), worker.StartConfig{})
	assert.NoError(t, err)

	r := &recordingReporter{}
	ctx := progress.ContextWithReporter(context.Background(), r)

	// mirrors exactly what rpcAdapter.Send does with a.sidecar
	a.sidecar.Bind("eval", progress.FromContext(ctx))
	defer a.sidecar.UnbindAfterGrace()

	resp, err := http.Post(a.sidecar.URL(), "application/json", strings.NewReader(`{"message":"checking correctness"}`))
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	assert.Eventually(t, func() bool {
		return len(r.recorded()) == 1
	}, time.Second, 5*time.Millisecond, "expected the worker's progress event to be relayed")

	events := r.recorded()
	assert.Equal(t, progress.StageEvaluating, events[0].Stage)
	assert.Equal(t, "eval", events[0].Command)
	assert.Equal(t, "checking correctness", events[0].Message)
}

// func TestStdioAdapter_Send(t *testing.T) {
// 	a, w := createStdioAdapter(t)

// 	ctx := context.Background()
// 	data := "test"

// 	w.EXPECT().Send(ctx, data, 0).Return("result", nil)

// 	res, err := a.Send(ctx, data, 0)
// 	assert.NoError(t, err)
// 	assert.Equal(t, "result", res)
// }

// func TestStdioAdapter_Send_PassesError(t *testing.T) {
// 	a, w := createStdioAdapter(t)

// 	ctx := context.Background()
// 	data := "test"

// 	w.EXPECT().Send(ctx, data, 0).Return("result", assert.AnError)

// 	_, err := a.Send(ctx, data, 0)
// 	assert.ErrorIs(t, err, assert.AnError)
// }
