package supervisor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"

	"github.com/ethereum/go-ethereum/rpc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"github.com/lambda-feedback/shimmy/internal/execution/worker"
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

type fakeRpcError struct {
	code    int
	message string
}

func (e fakeRpcError) Error() string  { return e.message }
func (e fakeRpcError) ErrorCode() int { return e.code }

// fakeRpcDataError carries data decoded from JSON, as go-ethereum's
// client does for the error's "data" field.
type fakeRpcDataError struct {
	fakeRpcError
	data any
}

func (e fakeRpcDataError) ErrorData() any { return e.data }

func TestRpcSendError_InvalidSubmission(t *testing.T) {
	err := rpcSendError(fakeRpcError{code: InvalidSubmissionRpcCode, message: "Failed to parse SymPy expression: A/(w*"})

	var invalidErr *InvalidSubmissionError
	assert.ErrorAs(t, err, &invalidErr)
	assert.Equal(t, "Failed to parse SymPy expression: A/(w*", invalidErr.Message)
}

func TestRpcSendError_UpstreamTimeout(t *testing.T) {
	err := rpcSendError(fakeRpcDataError{
		fakeRpcError: fakeRpcError{code: UpstreamErrorRpcCode, message: "The feedback service took too long to respond. Please try again."},
		data:         map[string]any{"code": "UPSTREAM_TIMEOUT"},
	})

	var upstreamErr *UpstreamError
	assert.ErrorAs(t, err, &upstreamErr)
	assert.True(t, upstreamErr.Timeout)
	assert.Equal(t, "The feedback service took too long to respond. Please try again.", upstreamErr.Message)
	assert.Equal(t, UpstreamTimeoutFileCode, upstreamErr.Code())
}

func TestRpcSendError_UpstreamUnavailable(t *testing.T) {
	err := rpcSendError(fakeRpcDataError{
		fakeRpcError: fakeRpcError{code: UpstreamErrorRpcCode, message: "The feedback service is temporarily unavailable. Please try again."},
		data:         map[string]any{"code": "UPSTREAM_UNAVAILABLE"},
	})

	var upstreamErr *UpstreamError
	assert.ErrorAs(t, err, &upstreamErr)
	assert.False(t, upstreamErr.Timeout)
	assert.Equal(t, "The feedback service is temporarily unavailable. Please try again.", upstreamErr.Message)
	assert.Equal(t, UpstreamUnavailableFileCode, upstreamErr.Code())
}

func TestRpcSendError_UpstreamWithoutDataCode(t *testing.T) {
	for name, rpcErr := range map[string]error{
		"no data":      fakeRpcError{code: UpstreamErrorRpcCode, message: "failed"},
		"data no code": fakeRpcDataError{fakeRpcError: fakeRpcError{code: UpstreamErrorRpcCode, message: "failed"}, data: map[string]any{}},
		"data string":  fakeRpcDataError{fakeRpcError: fakeRpcError{code: UpstreamErrorRpcCode, message: "failed"}, data: "UPSTREAM_TIMEOUT"},
	} {
		t.Run(name, func(t *testing.T) {
			err := rpcSendError(rpcErr)

			var upstreamErr *UpstreamError
			assert.ErrorAs(t, err, &upstreamErr)
			assert.False(t, upstreamErr.Timeout)
			assert.Equal(t, UpstreamUnavailableFileCode, upstreamErr.Code())
		})
	}
}

func TestRpcSendError_ServerErrorCodesAreGeneric(t *testing.T) {
	for _, code := range []int{503, 504} {
		err := rpcSendError(fakeRpcError{code: code, message: "boom"})

		_, retryable := RetryableErrorCode(err)
		assert.False(t, retryable)
		assert.ErrorContains(t, err, "error sending rpc request: boom")
	}
}

func TestRpcSendError_DeadlineExceeded(t *testing.T) {
	err := rpcSendError(context.DeadlineExceeded)

	var timeoutErr *FunctionTimeoutError
	assert.ErrorAs(t, err, &timeoutErr)
	assert.Equal(t, "The evaluation function did not respond in time.", timeoutErr.Message)
	assert.Equal(t, FunctionTimeoutCode, timeoutErr.Code())
}

func TestRpcSendError_OtherError(t *testing.T) {
	rpcErr := fakeRpcError{code: 0, message: "boom"}
	err := rpcSendError(rpcErr)

	var invalidErr *InvalidSubmissionError
	assert.False(t, errors.As(err, &invalidErr))
	_, retryable := RetryableErrorCode(err)
	assert.False(t, retryable)
	assert.ErrorIs(t, err, rpcErr)
}

func TestRpcSendError_InternalErrorCode(t *testing.T) {
	err := rpcSendError(fakeRpcError{code: 500, message: "boom"})

	_, retryable := RetryableErrorCode(err)
	assert.False(t, retryable)
	assert.ErrorContains(t, err, "error sending rpc request: boom")
}

// TestRpcSendError_ToolkitUpstreamResponse decodes the error response
// lf_toolkit sends for UpstreamTimeoutError through a real rpc client.
func TestRpcSendError_ToolkitUpstreamResponse(t *testing.T) {
	clientConn, serverConn := net.Pipe()

	go func() {
		var req struct {
			ID json.RawMessage `json:"id"`
		}
		if err := json.NewDecoder(serverConn).Decode(&req); err != nil {
			return
		}
		_, _ = fmt.Fprintf(serverConn,
			`{"jsonrpc":"2.0","id":%s,"error":{"code":424,"message":"timed out","data":{"code":"UPSTREAM_TIMEOUT"}}}`,
			req.ID)
	}()

	client, err := rpc.DialIO(context.Background(), clientConn, clientConn)
	assert.NoError(t, err)

	// close the pipe before the client, which waits for its read loop
	defer client.Close()
	defer clientConn.Close()
	defer serverConn.Close()

	var result map[string]any
	callErr := client.CallContext(context.Background(), &result, "eval", map[string]any{})
	err = rpcSendError(callErr)

	var upstreamErr *UpstreamError
	assert.ErrorAs(t, err, &upstreamErr)
	assert.True(t, upstreamErr.Timeout)
	assert.Equal(t, "timed out", upstreamErr.Message)
}
