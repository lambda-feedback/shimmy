package progress

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// sseStep is one progress step. The same shape is written both as its own
// live frame (event: <stage>) the moment the event arrives and as an
// element of the terminal envelope's steps[], so a client parses "a step"
// the same way whether it arrives inline or inside the terminal frame.
type sseStep struct {
	Stage     string         `json:"stage"`
	Message   string         `json:"message,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// sseEnvelope is the JSON payload of the single terminal SSE frame for an
// /evaluate (or /preview) request. The same shape is used for the
// "completed" and "failed" events: on failure Feedback is null and Error
// (an ErrorResponse-shaped object) carries the detail. It matches the
// spec's SseEvaluateTerminalFrame.
type sseEnvelope struct {
	Feedback []map[string]any `json:"feedback"`
	Steps    []sseStep        `json:"steps"`
	Error    *ErrorInfo       `json:"error,omitempty"`
}

// sseChatEnvelope is the terminal-frame payload for a /chat request. Chat
// has no feedback[]; it returns an output object plus optional metadata.
// On failure Output is null and Error carries the detail. It matches the
// spec's SseChatTerminalFrame.
type sseChatEnvelope struct {
	Output   map[string]any `json:"output"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Steps    []sseStep      `json:"steps"`
	Error    *ErrorInfo     `json:"error,omitempty"`
}

// SSEReporter is a Reporter that streams progress back to the caller on
// the /evaluate or /chat response itself, as Server-Sent Events. Each
// non-terminal event is written immediately as its own frame
// (event: <stage>, data = the step object) so the caller sees progress as
// it happens, and is also accumulated; on completion/failure a single
// terminal frame (event: completed | failed) carries the result plus
// every step that preceded it, then the handler closes the connection.
//
// Report is called concurrently — synchronously from the request
// goroutine for shim-authored events, and from detached sidecar
// goroutines for worker-authored sub-steps — so all state and all writes
// to the ResponseWriter are guarded by mu.
type SSEReporter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	// command ("evaluate" | "preview" | "chat") only selects the terminal
	// frame shape (feedback[] vs output/metadata); it is not serialised.
	command string
	log     *zap.Logger

	mu            sync.Mutex
	steps         []sseStep
	seenPreparing bool
	seenStarting  bool
	terminated    bool
	terminalOnce  sync.Once
}

var _ Reporter = (*SSEReporter)(nil)

// NewSSEReporter returns a reporter that writes SSE frames to w. It
// returns an error if w cannot be flushed incrementally, so the caller
// can fall back to a buffered response.
func NewSSEReporter(w http.ResponseWriter, command string, log *zap.Logger) (*SSEReporter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("response writer does not support flushing")
	}
	return &SSEReporter{
		w:       w,
		flusher: flusher,
		command: command,
		log:     log,
	}, nil
}

// Report streams a non-terminal event as its own frame (and accumulates
// it), or writes the single terminal frame. Once the terminal frame is
// written, all further events (including a late worker sub-step relayed
// after the request returned) are dropped without touching the
// ResponseWriter.
func (r *SSEReporter) Report(_ context.Context, evt Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.terminated {
		return
	}

	if evt.Stage.terminal() {
		r.terminalOnce.Do(func() {
			r.terminated = true
			r.writeEnvelopeLocked(evt)
		})
		return
	}

	// Collapse the shim's lifecycle markers to their first occurrence for
	// the whole request: the per-case evaluation loop re-enters the
	// supervisor and re-emits preparing/starting once per case.
	// Worker-authored sub-steps (evaluating / thinking) are never
	// collapsed — they sit on their own stages.
	switch evt.Stage {
	case StagePreparing:
		if r.seenPreparing {
			return
		}
		r.seenPreparing = true
	case StageStarting:
		if r.seenStarting {
			return
		}
		r.seenStarting = true
	}

	step := sseStep{
		Stage:     string(evt.Stage),
		Message:   evt.Message,
		Data:      evt.Data,
		Timestamp: evt.Timestamp,
	}
	if step.Timestamp.IsZero() {
		// Worker-authored sub-steps bypass Emit (they come off the
		// sidecar) and arrive without a timestamp.
		step.Timestamp = time.Now().UTC()
	}

	r.steps = append(r.steps, step)
	r.writeStepLocked(step)
}

// failureErrorInfo returns the ErrorResponse-shaped object for a "failed"
// terminal frame. It prefers the structured ErrorInfo the handler
// attached; failing that it synthesises a minimal object from the event's
// human-facing Message and raw Error so `title` is never empty.
func failureErrorInfo(evt Event) *ErrorInfo {
	if evt.ErrorInfo != nil {
		return evt.ErrorInfo
	}
	return &ErrorInfo{Title: "Error", Message: evt.Message, Trace: evt.Error}
}

func (r *SSEReporter) writeEnvelopeLocked(evt Event) {
	steps := r.steps
	if steps == nil {
		steps = []sseStep{}
	}
	failed := evt.Stage == StageFailed

	event := "completed"
	if failed {
		event = "failed"
	}

	var payload any
	if r.command == "chat" {
		env := sseChatEnvelope{Steps: steps}
		if failed {
			env.Error = failureErrorInfo(evt)
		} else {
			env.Output, _ = evt.Data["output"].(map[string]any)
			env.Metadata, _ = evt.Data["metadata"].(map[string]any)
		}
		payload = env
	} else {
		env := sseEnvelope{Steps: steps}
		if failed {
			env.Error = failureErrorInfo(evt)
		} else {
			feedback, ok := evt.Data["feedback"].([]map[string]any)
			if !ok {
				feedback = []map[string]any{}
			}
			env.Feedback = feedback
		}
		payload = env
	}

	body, err := json.Marshal(payload)
	if err != nil {
		r.log.Warn("failed to marshal SSE envelope", zap.String("event", event), zap.Error(err))
		return
	}

	if _, err := fmt.Fprintf(r.w, "event: %s\ndata: %s\n\n", event, body); err != nil {
		r.log.Debug("failed to write SSE terminal frame", zap.Error(err))
		return
	}
	r.flusher.Flush()
}

// writeStepLocked streams a single intermediate progress step as its own
// SSE frame (event: <stage>), so the caller sees progress as it happens
// rather than only in the terminal frame. The step is already recorded in
// r.steps for the terminal envelope, so a marshal or write failure here
// only costs the live frame. A write failure does not set terminated: the
// terminal-frame attempt and further accumulation continue. Callers hold
// r.mu.
func (r *SSEReporter) writeStepLocked(step sseStep) {
	body, err := json.Marshal(step)
	if err != nil {
		r.log.Warn("failed to marshal SSE step", zap.String("stage", step.Stage), zap.Error(err))
		return
	}

	if _, err := fmt.Fprintf(r.w, "event: %s\ndata: %s\n\n", step.Stage, body); err != nil {
		r.log.Debug("failed to write SSE step frame", zap.Error(err))
		return
	}
	r.flusher.Flush()
}

// Heartbeat writes an SSE comment line to keep the connection alive. It
// is a no-op once the terminal frame has been written.
func (r *SSEReporter) Heartbeat() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.terminated {
		return
	}
	if _, err := r.w.Write([]byte(": ping\n\n")); err != nil {
		r.log.Debug("failed to write SSE heartbeat", zap.Error(err))
		return
	}
	r.flusher.Flush()
}
