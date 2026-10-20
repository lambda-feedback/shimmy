package supervisor

import "errors"

var (
	ErrUnsupportedIOInterface = errors.New("unsupported io interface")
	ErrUnsupportedIOTransport = errors.New("unsupported io transport")
)

const (
	// InvalidSubmissionRpcCode is the JSON-RPC error code an rpc worker
	// returns when the evaluation function cannot process the submission.
	InvalidSubmissionRpcCode = 422

	// InvalidSubmissionFileCode is the error code a file worker returns
	// when the evaluation function cannot process the submission.
	InvalidSubmissionFileCode = "INVALID_SUBMISSION"

	// UpstreamErrorRpcCode is the JSON-RPC error code an rpc worker
	// returns when a service the evaluation function depends on timed out
	// or failed. The error's data carries the file code below, e.g.
	// {"code": "UPSTREAM_TIMEOUT"}, to tell the two cases apart.
	UpstreamErrorRpcCode = 424

	// UpstreamTimeoutFileCode is the error code a worker returns when a
	// service the evaluation function depends on did not answer in time.
	UpstreamTimeoutFileCode = "UPSTREAM_TIMEOUT"

	// UpstreamUnavailableFileCode is the error code a worker returns when
	// a service the evaluation function depends on refused or failed the
	// request.
	UpstreamUnavailableFileCode = "UPSTREAM_UNAVAILABLE"

	// FunctionTimeoutCode is the error code reported when the worker did
	// not respond within the send timeout.
	FunctionTimeoutCode = "FUNCTION_TIMEOUT"
)

// InvalidSubmissionError is returned when the worker reports that the
// evaluation function cannot process the submitted response, e.g. due
// to an unparseable expression.
type InvalidSubmissionError struct {
	Message string
}

func (e *InvalidSubmissionError) Error() string {
	return e.Message
}

// UpstreamError is returned when the worker reports that a service the
// evaluation function depends on timed out or was unavailable. The
// request may succeed if retried.
type UpstreamError struct {
	Message string
	Timeout bool
}

func (e *UpstreamError) Error() string {
	return e.Message
}

// Code returns the stable error code reported to clients.
func (e *UpstreamError) Code() string {
	if e.Timeout {
		return UpstreamTimeoutFileCode
	}
	return UpstreamUnavailableFileCode
}

// FunctionTimeoutError is returned when the worker did not respond
// within the send timeout. The request may succeed if retried.
type FunctionTimeoutError struct {
	Message string
}

func (e *FunctionTimeoutError) Error() string {
	return e.Message
}

// Code returns the stable error code reported to clients.
func (e *FunctionTimeoutError) Code() string {
	return FunctionTimeoutCode
}

// RetryableErrorCode reports the stable error code of err if it, or any
// error it wraps, is an UpstreamError or a FunctionTimeoutError.
func RetryableErrorCode(err error) (string, bool) {
	var upstreamErr *UpstreamError
	if errors.As(err, &upstreamErr) {
		return upstreamErr.Code(), true
	}

	var timeoutErr *FunctionTimeoutError
	if errors.As(err, &timeoutErr) {
		return timeoutErr.Code(), true
	}

	return "", false
}

// RetryableErrorMessage returns the message of the UpstreamError or
// FunctionTimeoutError wrapped by err, without the context added while
// the error travelled up from the worker.
func RetryableErrorMessage(err error) (string, bool) {
	var upstreamErr *UpstreamError
	if errors.As(err, &upstreamErr) {
		return upstreamErr.Message, true
	}

	var timeoutErr *FunctionTimeoutError
	if errors.As(err, &timeoutErr) {
		return timeoutErr.Message, true
	}

	return "", false
}

// IOInterface describes the interface used to communicate with the worker
type IOInterface string

const (
	// RpcIO describes communication w/ processes over rpc
	RpcIO IOInterface = "rpc"

	// FileIO describes communication w/ processes over files
	FileIO IOInterface = "file"
)

// IOTransport describes the transport mechanism used to communicate with
type IOTransport string

const (
	// IpcTransport describes communication w/ processes over IPC.
	// This can be unix sockets or windows named pipes, depending on the OS.
	IpcTransport IOTransport = "ipc"

	// Http describes communication w/ processes over http
	HttpTransport IOTransport = "http"

	// Stdio describes communication w/ processes over stdio
	StdioTransport IOTransport = "stdio"

	// Ws describes communication w/ processes over websockets
	WsTransport IOTransport = "ws"

	// Tcp describes communication w/ processes over tcp
	TcpTransport IOTransport = "tcp"
)
