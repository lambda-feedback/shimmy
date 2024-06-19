package supervisor

import "errors"

var (
	ErrUnsupportedIOInterface  = errors.New("unsupported io interface")
	ErrUnsupportedIOTransport  = errors.New("unsupported io transport")
	ErrInvalidPersistentFileIO = errors.New("persistent workers are not supported for file IO yet")
)

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
	// IPCTransport describes communication w/ processes over IPC.
	// This can be unix sockets or windows named pipes, depending on the OS.
	IPCTransport IOTransport = "unix"

	// Http describes communication w/ processes over http
	HttpTransport IOTransport = "http"

	// Stdio describes communication w/ processes over stdio
	StdioTransport IOTransport = "stdio"

	// Ws describes communication w/ processes over websockets
	WsTransport IOTransport = "ws"
)
