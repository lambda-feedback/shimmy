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

// sseStep is one accumulated progress step in the final SSE frame. Its
// shape is deliberately the same one a future mid-stream frame will use,
// so a client parses "a step" the same way whether it arrives inline or
// inside the terminal envelope.
type sseStep struct {
	Stage     string         `json:"stage"`
	Message   string         `json:"message,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// sseEnvelope is the JSON payload of the single terminal SSE frame. The
// same shape is used for the "completed" and "failed" events: on failure
// Feedback is null and Error/Message carry the detail.
type sseEnvelope struct {
	Command  string           `json:"command"`
	Feedback []map[string]any `json:"feedback"`
	Steps    []sseStep        `json:"steps"`
	Error    string           `json:"error,omitempty"`
	Message  string           `json:"message,omitempty"`
}

// SSEReporter is a Reporter that streams progress back to the caller on
// the /evaluate response itself, as Server-Sent Events. In this phase it
// silently accumulates the intermediate steps and emits exactly one
// terminal frame (event: completed | failed) carrying the feedback plus
// every step that preceded it, then the handler closes the connection.
//
// Report is called concurrently — synchronously from the request
// goroutine for shim-authored events, and from detached sidecar
// goroutines for worker-authored "progress" events — so all state and
// all writes to the ResponseWriter are guarded by mu.
type SSEReporter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	command string
	log     *zap.Logger

	mu           sync.Mutex
	steps        []sseStep
	terminated   bool
	terminalOnce sync.Once
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

// Report accumulates a non-terminal event as a step, or writes the single
// terminal frame. Once the terminal frame is written, all further events
// (including a late worker "progress" relayed after the request returned)
// are dropped without touching the ResponseWriter.
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

	step := sseStep{
		Stage:     string(evt.Stage),
		Message:   evt.Message,
		Data:      evt.Data,
		Timestamp: evt.Timestamp,
	}
	if step.Timestamp.IsZero() {
		// Worker-authored "progress" events bypass Emit and arrive
		// without a timestamp.
		step.Timestamp = time.Now().UTC()
	}

	// Collapse a run of identical lifecycle stages — the per-case
	// evaluation loop re-enters the supervisor and re-emits
	// preparing/evaluating each time. "progress" steps are never
	// collapsed.
	if n := len(r.steps); n > 0 && r.steps[n-1].Stage == step.Stage &&
		(evt.Stage == StagePreparing || evt.Stage == StageEvaluating) {
		return
	}

	r.steps = append(r.steps, step)
}

func (r *SSEReporter) writeEnvelopeLocked(evt Event) {
	env := sseEnvelope{
		Command: r.command,
		Steps:   r.steps,
	}
	if env.Steps == nil {
		env.Steps = []sseStep{}
	}

	event := "completed"
	if evt.Stage == StageFailed {
		event = "failed"
		env.Feedback = nil
		env.Error = evt.Error
		env.Message = evt.Message
	} else {
		feedback, ok := evt.Data["feedback"].([]map[string]any)
		if !ok {
			feedback = []map[string]any{}
		}
		env.Feedback = feedback
	}

	body, err := json.Marshal(env)
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
