package progress

import "time"

// Stage identifies a point in the lifecycle of an evaluation request that
// progress events can be emitted for.
type Stage string

const (
	// StagePreparing indicates the evaluation environment is being set up
	// (a worker is ready to receive work, whether freshly booted or reused
	// from a warm pool). Deliberately named around what a student or
	// teacher would recognise, not shimmy's internal "worker" concept.
	// Emitted by shimmy itself, once per request.
	StagePreparing Stage = "preparing"

	// StageStarting indicates the worker is about to be invoked. Emitted by
	// shimmy itself, once per request, for both /evaluate and /chat.
	StageStarting Stage = "starting"

	// StageEvaluating indicates a worker-authored sub-step during an
	// /evaluate (or /preview) request, relayed from the worker's local
	// progress side-channel (see Sidecar). A worker cannot claim any stage;
	// the sidecar assigns this based on the command in flight.
	StageEvaluating Stage = "evaluating"

	// StageThinking indicates a worker-authored sub-step during a /chat
	// request, relayed from the worker's local progress side-channel (see
	// Sidecar). Like StageEvaluating, the sidecar assigns it by command;
	// the worker cannot set it.
	StageThinking Stage = "thinking"

	// StageCompleted indicates feedback has been computed and is about
	// to be returned to the caller.
	StageCompleted Stage = "completed"

	// StageFailed indicates a terminal failure at any layer of the pipeline.
	StageFailed Stage = "failed"

	// StageProgress is retained for compatibility but is no longer emitted:
	// worker-authored sub-steps are now relayed as StageEvaluating or
	// StageThinking depending on the command in flight (see Sidecar).
	StageProgress Stage = "progress"
)

// terminal reports whether the stage marks the end of an evaluation's
// progress event stream. At most one terminal event is delivered per
// Reporter instance.
func (s Stage) terminal() bool {
	return s == StageCompleted || s == StageFailed
}

// ErrorInfo is structured failure detail for a StageFailed event. On the
// SSE terminal "failed" frame it is emitted as the frame's `error`
// object, shaped like the µEd spec's ErrorResponse (title is required;
// the rest are optional). It carries no student/teacher-facing copy —
// that stays on Event.Message.
type ErrorInfo struct {
	Title   string         `json:"title"`
	Message string         `json:"message,omitempty"`
	Code    string         `json:"code,omitempty"`
	Trace   string         `json:"trace,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

// Event describes a single progress update for an evaluation request.
type Event struct {
	// Stage is the lifecycle point this event describes.
	Stage Stage

	// Command is the µEd command being processed (e.g. "eval", "preview").
	Command string

	// Message is a short, student/teacher-facing description of this
	// event, safe to display as-is (e.g. "Evaluating your submission…").
	// It must never contain raw technical error detail — see Error.
	Message string

	// Error carries raw technical error detail for StageFailed events,
	// intended for logs/support diagnostics. Never display this to
	// students or teachers directly; show Message instead.
	Error string

	// ErrorInfo is the structured failure detail for a StageFailed event.
	// The SSE reporter emits it as the terminal "failed" frame's `error`
	// object; when nil it falls back to a minimal object built from
	// Message/Error. Ignored by non-SSE reporters.
	ErrorInfo *ErrorInfo

	// Data is a free-form extension point. On StageCompleted it carries
	// the final result payload (so a callbackUrl-supplying caller gets the
	// result, not just a status ping). On a worker-authored sub-step
	// (StageEvaluating / StageThinking) it carries whatever the evaluation
	// function attached to its custom event (see Sidecar).
	Data map[string]any

	// Timestamp is set by Emit, not by callers.
	Timestamp time.Time
}
