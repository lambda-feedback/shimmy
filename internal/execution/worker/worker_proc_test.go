package worker_test

import (
	"context"
	"syscall"
	"testing"
	"time"

	"github.com/lambda-feedback/shimmy/internal/execution/worker"
	"github.com/lambda-feedback/shimmy/util"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestWorker_Start_IsAlive(t *testing.T) {
	w := worker.NewProcessWorker[any, any](zap.NewNop())

	err := w.Start(context.Background(), worker.StartConfig{
		Cmd: "cat",
	})
	assert.NoError(t, err)

	defer w.Stop(context.Background())

	time.Sleep(1000 * time.Millisecond)

	assert.Equal(t, true, util.IsProcessAlive(w.Pid()))
}

func TestWorker_Start_FailsIfStarted(t *testing.T) {
	w := worker.NewProcessWorker[any, any](zap.NewNop())

	startConfig := worker.StartConfig{
		Cmd: "cat",
	}

	err := w.Start(context.Background(), startConfig)
	assert.NoError(t, err)

	defer w.Stop(context.Background())

	err = w.Start(context.Background(), startConfig)
	assert.Error(t, err)
}

func TestWorker_Start_FailsIfContextCancelled(t *testing.T) {
	w := worker.NewProcessWorker[any, any](zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := w.Start(ctx, worker.StartConfig{
		Cmd: "cat",
	})
	assert.Error(t, err)
}

func TestWorker_TerminatesIfContextCancelled(t *testing.T) {
	w := worker.NewProcessWorker[any, any](zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())

	err := w.Start(ctx, worker.StartConfig{
		Cmd: "cat",
	})
	assert.NoError(t, err)

	// cancel the worker context
	cancel()

	evt, err := w.Wait(context.Background())
	assert.NotNil(t, evt)
	assert.NoError(t, err)

	// the process should have been terminated w/ a sigkill in the background
	if evt, ok := evt.(*worker.ProcessExitEvent); ok {
		assert.Equal(t, syscall.SIGKILL, syscall.Signal(*evt.Signal))
		assert.Nil(t, evt.Code)
	} else {
		t.Errorf("unexpected event type: %T", evt)
	}
}

func TestWorker_CapturesStderr(t *testing.T) {
	w := worker.NewProcessWorker[any, any](zap.NewNop())

	err := w.Start(context.Background(), worker.StartConfig{
		Cmd:  "sh",
		Args: []string{"-c", ">&2 echo \"error\""},
	})
	assert.NoError(t, err)

	defer w.Stop(context.Background())

	evt, err := w.Wait(context.Background())
	assert.NotNil(t, evt)
	assert.NoError(t, err)

	if evt, ok := evt.(*worker.ProcessExitEvent); ok {
		assert.Equal(t, 0, *evt.Code)
		assert.Equal(t, "error\n", evt.Stderr)
	} else {
		t.Errorf("unexpected event type: %T", evt)
	}
}

func TestWorker_Wait_ReturnsExitEvent(t *testing.T) {
	w := worker.NewProcessWorker[any, any](zap.NewNop())

	err := w.Start(context.Background(), worker.StartConfig{
		Cmd: "echo",
	})
	assert.NoError(t, err)

	defer w.Stop(context.Background())

	evt, err := w.Wait(context.Background())
	assert.NotNil(t, evt)
	assert.NoError(t, err)

	if evt, ok := evt.(*worker.ProcessExitEvent); ok {
		assert.Equal(t, 0, *evt.Code)
		assert.Nil(t, evt.Signal)
	} else {
		t.Errorf("unexpected event type: %T", evt)
	}
}

func TestWorker_Wait_ReturnsErrorIfContextCancelled(t *testing.T) {
	w := worker.NewProcessWorker[any, any](zap.NewNop())

	err := w.Start(context.Background(), worker.StartConfig{
		Cmd: "cat",
	})
	assert.NoError(t, err)

	defer w.Stop(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = w.Wait(ctx)
	assert.Error(t, err)
}

func TestWorker_WaitFor_ReturnsExitEvent(t *testing.T) {
	w := worker.NewProcessWorker[any, any](zap.NewNop())

	err := w.Start(context.Background(), worker.StartConfig{
		Cmd: "echo",
	})
	assert.NoError(t, err)

	defer w.Stop(context.Background())

	evt, err := w.WaitFor(context.Background(), 0)
	assert.NoError(t, err)

	if evt, ok := evt.(*worker.ProcessExitEvent); ok {
		assert.Equal(t, 0, *evt.Code)
		assert.Nil(t, evt.Signal)
	} else {
		t.Errorf("unexpected event type: %T", evt)
	}
}

func TestWorker_WaitFor_ReturnsErrorIfTimeout(t *testing.T) {
	w := worker.NewProcessWorker[any, any](zap.NewNop())

	err := w.Start(context.Background(), worker.StartConfig{
		Cmd:  "sleep",
		Args: []string{"1"},
	})
	assert.NoError(t, err)

	defer w.Stop(context.Background())

	_, err = w.WaitFor(context.Background(), 100*time.Millisecond)
	assert.Error(t, err)
}

func TestWorker_Stop_TerminatesProcess(t *testing.T) {
	w := worker.NewProcessWorker[any, any](zap.NewNop())

	err := w.Start(context.Background(), worker.StartConfig{
		Cmd: "cat",
	})
	assert.NoError(t, err)

	w.Stop(context.Background())

	evt, err := w.Wait(context.Background())
	assert.NotNil(t, evt)
	assert.NoError(t, err)

	if evt, ok := evt.(*worker.ProcessExitEvent); ok {
		// the process should have been terminated w/ a sigterm in the background
		assert.Equal(t, syscall.SIGTERM, syscall.Signal(*evt.Signal))
		assert.Nil(t, evt.Code)
	} else {
		t.Errorf("unexpected event type: %T", evt)
	}

	// the process should not be alive
	assert.Equal(t, false, util.IsProcessAlive(w.Pid()))
}
