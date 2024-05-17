package worker

import (
	context "context"
	"time"
)

// ExitStatus represents the exit status of a worker
type ExitStatus int

const (
	// ExitStatusSuccess indicates that the worker exited successfully
	ExitStatusSuccess ExitStatus = iota

	// ExitStatusFailure indicates that the worker exited with an error
	ExitStatusFailure
)

// ExitEvent represents an event that indicates the exit status of a worker
type ExitEvent interface {
	// Status returns the exit status of the worker
	Status() ExitStatus
}

type Worker[I, O any] interface {
	Start(context.Context, StartConfig) error
	Stop(context.Context) error
	Wait(context.Context) (ExitEvent, error)
	WaitFor(context.Context, time.Duration) (ExitEvent, error)
}
