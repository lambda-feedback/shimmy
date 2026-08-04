package progress

import "time"

// Stage identifies a point in the lifecycle of an evaluation request that
// progress events can be emitted for.
type Stage string

const (
	// StageWorkerAcquired indicates a worker is ready to receive work,
	// whether it was freshly booted or reused from a warm pool.
	StageWorkerAcquired Stage = "worker_acquired"

	// StageRunning indicates the evaluation function is about to be invoked.
	StageRunning Stage = "running"

	// StageFeedbackReady indicates feedback has been computed and is about
	// to be returned to the caller.
	StageFeedbackReady Stage = "feedback_ready"

	// StageFailed indicates a terminal failure at any layer of the pipeline.
	StageFailed Stage = "failed"
)

// terminal reports whether the stage marks the end of an evaluation's
// progress event stream. At most one terminal event is delivered per
// Reporter instance.
func (s Stage) terminal() bool {
	return s == StageFeedbackReady || s == StageFailed
}

// Event describes a single progress update for an evaluation request.
type Event struct {
	// Stage is the lifecycle point this event describes.
	Stage Stage

	// Command is the µEd command being processed (e.g. "eval", "preview").
	Command string

	// Message is an optional human-readable note.
	Message string

	// Error is populated only for StageFailed.
	Error string

	// Data is a free-form extension point, reserved for future events
	// (e.g. ones emitted by the evaluation function process itself).
	Data map[string]any

	// Timestamp is set by Emit, not by callers.
	Timestamp time.Time
}
