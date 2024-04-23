package worker

import (
	"fmt"
	"time"
)

var (
	ErrKillTimeout          = fmt.Errorf("kill timeout")
	ErrInvalidTimeout       = fmt.Errorf("invalid timeout")
	ErrWorkerNotStarted     = fmt.Errorf("worker not started")
	ErrWorkerAlreadyStarted = fmt.Errorf("worker already started")
)

type StartConfig struct {
	// Cmd is the path or name of the binary to execute
	Cmd string `conf:"cmd"`

	// Cwd is the working directory in which
	// the binary should be executed
	Cwd string `conf:"cwd"`

	// Args is the list of arguments to pass to the command
	Args []string `conf:"arg"`

	// Env is a map of environment variables
	// to set when running the command
	Env map[string]string `conf:"env"`
}

type StopConfig struct {
	// Timeout is the duration to wait for the worker to stop
	Timeout time.Duration `conf:"timeout"`
}

type SendConfig struct {
	// Timeout is the duration to wait for the worker to send a message
	Timeout time.Duration `conf:"timeout"`
	// CloseAfterSend indicates whether to close the channel after sending
	CloseAfterSend bool `conf:"close_after_send"`
}

type ReadConfig struct {
	// Timeout is the duration to wait for the worker to send a message
	Timeout time.Duration `conf:"timeout"`
}

type Request[T, M any] struct {
	// ID is the message identifier
	ID int `json:"id,omitempty"`

	// Data is the message payload
	Data T `json:"data"`

	// Meta is the message metadata
	Meta M `json:"meta,omitempty"`
}

type Response[T any] struct {
	// ID is the message identifier
	ID int `json:"id"`

	// Data is the message payload
	Data T `json:"data"`
}
