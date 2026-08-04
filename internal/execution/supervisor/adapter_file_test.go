package supervisor

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/lambda-feedback/shimmy/internal/execution/worker"
	"github.com/lambda-feedback/shimmy/internal/progress"
)

// recordingReporter is a minimal progress.Reporter test double, local to
// this package since progress.Reporter's own test double is unexported
// in a different package.
type recordingReporter struct {
	mu     sync.Mutex
	events []progress.Event
}

func (r *recordingReporter) Report(_ context.Context, evt progress.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, evt)
}

func (r *recordingReporter) recorded() []progress.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]progress.Event(nil), r.events...)
}

// envValue returns the value of the first "key=value" entry in env, or ""
// if key isn't present.
func envValue(env []string, key string) string {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return strings.TrimPrefix(e, prefix)
		}
	}
	return ""
}

func TestFileAdapter_Start_DoesNotStartWorker(t *testing.T) {
	a, w := createFileAdapter(t)

	err := a.Start(context.Background(), worker.StartConfig{})
	assert.NoError(t, err)

	w.AssertNotCalled(t, "Start")
}

func TestFileAdapter_Stop_DoesNotStopWorker(t *testing.T) {
	a, w := createFileAdapter(t)

	_, err := a.Stop()
	assert.NoError(t, err)

	w.AssertNotCalled(t, "Terminate")
}

func TestFileAdapter_Send(t *testing.T) {
	w := worker.NewMockWorker(t)

	var sp *worker.StartConfig

	workerFactory := func(params worker.StartConfig) (worker.Worker, error) {
		sp = &params
		return w, nil
	}

	a := &fileAdapter{
		workerFactory: workerFactory,
		log:           zap.NewNop(),
	}

	ctx := context.Background()
	data := map[string]any{"foo": "bar"}

	var requestFileName string
	var responseFileName string

	// for the adapter to succeed, the worker process must write to
	// the response file before exiting. we mock this behaviour here.
	w.EXPECT().Start(mock.Anything).RunAndReturn(func(ctx context.Context) error {
		requestFileName = sp.Args[len(sp.Args)-2]
		responseFileName = sp.Args[len(sp.Args)-1]
		data, _ := os.ReadFile(requestFileName)
		_ = os.WriteFile(responseFileName, data, os.ModeAppend)
		return nil
	})
	w.EXPECT().ReadPipe().Return(io.NopCloser(strings.NewReader("")), nil)
	var cell int
	w.EXPECT().Wait(mock.Anything).Return(worker.ExitEvent{Code: &cell}, nil)

	res, err := a.Send(ctx, "test", data, 10)
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{"command": "test", "params": data}, res)

	// check that the request and response files were cleaned up
	_, err = os.Stat(requestFileName)
	assert.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(responseFileName)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestFileAdapter_Send_ReturnsStartError(t *testing.T) {
	a, w := createFileAdapter(t)

	ctx := context.Background()
	data := map[string]any{"foo": "bar"}

	w.EXPECT().ReadPipe().Return(io.NopCloser(strings.NewReader("")), nil)
	w.EXPECT().Start(mock.Anything).Return(assert.AnError)

	_, err := a.Send(ctx, "test", data, 0)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestFileAdapter_Send_ReturnsWaitForError(t *testing.T) {
	a, w := createFileAdapter(t)

	ctx := context.Background()
	data := map[string]any{"foo": "bar"}

	w.EXPECT().Start(mock.Anything).Return(nil)
	w.EXPECT().ReadPipe().Return(io.NopCloser(strings.NewReader("")), nil)
	w.EXPECT().Wait(mock.Anything).Return(worker.ExitEvent{}, assert.AnError)

	_, err := a.Send(ctx, "test", data, 0)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestFileAdapter_Send_ReturnsReadError(t *testing.T) {
	a, w := createFileAdapter(t)

	ctx := context.Background()
	data := map[string]any{"foo": "bar"}

	w.EXPECT().Start(mock.Anything).Return(nil)
	w.EXPECT().ReadPipe().Return(io.NopCloser(strings.NewReader("")), nil)
	var cell int
	w.EXPECT().Wait(mock.Anything).Return(worker.ExitEvent{Code: &cell}, nil)

	_, err := a.Send(ctx, "test", data, 0)
	assert.ErrorIs(t, err, io.EOF)
}

func TestFileAdapter_Send_ReturnsInvalidDataError(t *testing.T) {
	a, w := createFileAdapter(t)

	ctx := context.Background()
	data := map[string]any{"foo": make(chan int)}

	res, err := a.Send(ctx, "test", data, 0)
	assert.Error(t, err)
	assert.Nil(t, res)

	w.AssertNotCalled(t, "Start")
}

func TestFileAdapter_Send_InjectsProgressURLAndRelaysWorkerEvents(t *testing.T) {
	w := worker.NewMockWorker(t)

	var sp *worker.StartConfig
	workerFactory := func(params worker.StartConfig) (worker.Worker, error) {
		sp = &params
		return w, nil
	}

	a := &fileAdapter{
		workerFactory: workerFactory,
		log:           zap.NewNop(),
	}

	r := &recordingReporter{}
	ctx := progress.ContextWithReporter(context.Background(), r)
	data := map[string]any{"foo": "bar"}

	w.EXPECT().Start(mock.Anything).RunAndReturn(func(ctx context.Context) error {
		progressURL := envValue(sp.Env, "EVAL_PROGRESS_URL")
		require.NotEmpty(t, progressURL, "expected EVAL_PROGRESS_URL in worker env")

		resp, err := http.Post(progressURL, "application/json", strings.NewReader(`{"message":"checking correctness"}`))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusAccepted, resp.StatusCode)

		requestFileName := sp.Args[len(sp.Args)-2]
		responseFileName := sp.Args[len(sp.Args)-1]
		reqData, _ := os.ReadFile(requestFileName)
		_ = os.WriteFile(responseFileName, reqData, os.ModeAppend)
		return nil
	})
	w.EXPECT().ReadPipe().Return(io.NopCloser(strings.NewReader("")), nil)
	var cell int
	w.EXPECT().Wait(mock.Anything).Return(worker.ExitEvent{Code: &cell}, nil)

	_, err := a.Send(ctx, "eval", data, 10)
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		return len(r.recorded()) == 1
	}, time.Second, 5*time.Millisecond, "expected the worker's progress event to be relayed")

	events := r.recorded()
	assert.Equal(t, progress.StageProgress, events[0].Stage)
	assert.Equal(t, "eval", events[0].Command)
	assert.Equal(t, "checking correctness", events[0].Message)
}

func createFileAdapter(t *testing.T) (*fileAdapter, *worker.MockWorker) {
	w := worker.NewMockWorker(t)

	workerFactory := func(params worker.StartConfig) (worker.Worker, error) {
		return w, nil
	}

	adapter := &fileAdapter{
		workerFactory: workerFactory,
		log:           zap.NewNop(),
	}

	return adapter, w
}
