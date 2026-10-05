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
