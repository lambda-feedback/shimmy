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
	StagePreparing Stage = "preparing"

	// StageEvaluating indicates the submission is being evaluated.
	StageEvaluating Stage = "evaluating"

	// StageCompleted indicates feedback has been computed and is about
	// to be returned to the caller.
	StageCompleted Stage = "completed"

	// StageFailed indicates a terminal failure at any layer of the pipeline.
	StageFailed Stage = "failed"

	// StageProgress indicates a custom, evaluation-function-authored
	// progress update. Unlike the other stages, these are never emitted
	// by shimmy itself — only relayed from a worker's local progress
	// side-channel (see Sidecar). A worker cannot claim any other stage;
	// the wire contract for that side-channel has no way to set Stage.
	StageProgress Stage = "progress"
)

// terminal reports whether the stage marks the end of an evaluation's
// progress event stream. At most one terminal event is delivered per
// Reporter instance.
func (s Stage) terminal() bool {
	return s == StageCompleted || s == StageFailed
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

	// Data is a free-form extension point. On StageCompleted it carries
	// the evaluation's feedback payload (so a callbackUrl-supplying
	// caller gets the final result, not just a status ping). On
	// StageProgress it carries whatever the evaluation function attached
	// to its custom event (see Sidecar).
	Data map[string]any

	// Timestamp is set by Emit, not by callers.
	Timestamp time.Time
}
