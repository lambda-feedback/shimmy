package supervisor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/lambda-feedback/shimmy/internal/execution/worker"
	"go.uber.org/zap"
)

// RpcConfig describes the configuration for the rpc interface.
type RpcConfig struct {
	// Transport describes the transport mechanism used to communicate
	// with the worker. Only valid for rpc-based workers. Default is "stdio".
	//
	// If "stdio", the supervisor will communicate with the worker over
	// stdio. The worker is expected to read incoming messages from stdin
	// and write responses to stdout.
	//
	// If "ipc", the supervisor will communicate with the worker over
	// unix sockets or windows named pipes, depending on the OS. The
	// worker is expected to listen on a unix socket / pipe and handle
	// incoming messages.
	//
	// If "http", the supervisor will communicate with the worker over
	// http. The worker is expected to listen on a specified port and
	// handle incoming http requests.
	//
	// If "ws", the supervisor will communicate with the worker over
	// websockets. The worker is expected to listen on a specified port
	// and handle incoming websocket messages.
	Transport IOTransport `conf:"transport"`

	// HttpTransport is the configuration for the http transport.
	Http HttpTransportConfig `conf:"http"`

	// IPCTransportConfig is the configuration for the IPC transport.
	IPC IPCTransportConfig `conf:"ipc"`

	// WsTransport is the configuration for the websocket transport.
	Ws WsTransportConfig `conf:"ws"`
}

// HttpTransportConfig describes the configuration for http transport.
type HttpTransportConfig struct {
	// Url is the url to send http requests to.
	Url string `conf:"url"`
}

// IPCTransportConfig describes the configuration for unix socket transport.
type IPCTransportConfig struct {
	// Endpoint is the full path to the unix socket or
	// the name of the windows named pipe.
	Endpoint string `conf:"path"`
}

// WsTransportConfig describes the configuration for websocket transport.
type WsTransportConfig struct {
	// Url is the url to connect to.
	Url string `conf:"url"`
}

type rpcAdapter struct {
	workerFactory AdapterWorkerFactoryFn

	worker worker.Worker

	// stdioPipe is the stdio pipe used to communicate with the worker.
	// It is only set if the transport is "stdio".
	stdioPipe io.ReadWriteCloser

	// rpcClient is the rpc client used to communicate with the worker.
	rpcClient *rpc.Client

	config RpcConfig
	log    *zap.Logger
}

func newRpcAdapter(
	workerFactory AdapterWorkerFactoryFn,
	config RpcConfig,
	log *zap.Logger,
) *rpcAdapter {
	return &rpcAdapter{
		workerFactory: workerFactory,
		config:        config,
		log:           log.Named("adapter_rpc"),
	}
}

func (a *rpcAdapter) Start(
	ctx context.Context,
	params worker.StartConfig,
) error {
	if a.workerFactory == nil {
		return errors.New("no worker factory provided")
	}

	// create the worker
	worker, err := a.workerFactory(ctx, params)
	if err != nil {
		return fmt.Errorf("error creating worker: %w", err)
	}

	a.worker = worker

	// initialize the stdio pipe if the transport is "stdio"
	if a.config.Transport == StdioTransport {
		stdio, err := a.worker.DuplexPipe()
		if err != nil {
			return fmt.Errorf("error creating duplex pipe: %w", err)
		}

		// wrap the pipe in a header stream
		a.stdioPipe = &headerPrefixPipe{stdio: stdio}

		// TODO: close pipe?
	}

	// for rpc, we can already start the worker, as we do not need to pass
	// any additional, message-specific data to the worker via arguments
	if err := worker.Start(ctx); err != nil {
		return fmt.Errorf("error starting worker: %w", err)
	}

	// dial the rpc client
	if client, err := a.dialRpc(ctx, a.config); err != nil {
		return fmt.Errorf("error dialing rpc: %w", err)
	} else {
		a.rpcClient = client
	}

	return nil
}

func (a *rpcAdapter) Send(
	ctx context.Context,
	result any,
	method string,
	data map[string]any,
	timeout time.Duration,
) error {
	if a.worker == nil {
		return errors.New("no worker provided")
	}

	if a.rpcClient == nil {
		return errors.New("rpc client not available")
	}

	return a.rpcClient.CallContext(ctx, result, method, []any{data})
}

func (a *rpcAdapter) Stop(
	params worker.StopConfig,
) (ReleaseFunc, error) {
	if a.worker == nil {
		return nil, errors.New("no worker provided")
	}

	return stopWorker(a.worker, params)
}

// dialRpc dials the rpc client based on the given configuration.
func (a *rpcAdapter) dialRpc(
	ctx context.Context,
	config RpcConfig,
) (*rpc.Client, error) {
	if a.worker == nil {
		return nil, errors.New("worker not available")
	}

	switch config.Transport {
	case StdioTransport:
		if a.stdioPipe == nil {
			return nil, errors.New("stdio pipe not available")
		}

		return rpc.DialIO(ctx, a.stdioPipe, a.stdioPipe)
	case IPCTransport:
		return rpc.DialIPC(ctx, config.IPC.Endpoint)
	case HttpTransport:
		// TODO: use custom client
		return rpc.DialHTTP(config.Http.Url)
	case WsTransport:
		// TODO: use custom dialer
		// TODO: do we need to set custom origin?
		return rpc.DialWebsocket(ctx, config.Ws.Url, "")
	}

	return nil, ErrUnsupportedIOTransport
}
