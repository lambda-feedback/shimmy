package progress

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

type sseFrame struct {
	event string
	data  map[string]any
	raw   string
}

func parseSSEFrames(t *testing.T, raw string) []sseFrame {
	t.Helper()
	var frames []sseFrame
	for _, block := range strings.Split(strings.TrimSpace(raw), "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" || strings.HasPrefix(block, ":") {
			continue // heartbeat / comment
		}
		var f sseFrame
		f.raw = block
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "event: "):
				f.event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				payload := strings.TrimPrefix(line, "data: ")
				if err := json.Unmarshal([]byte(payload), &f.data); err != nil {
					t.Fatalf("frame data is not valid JSON: %v\n%s", err, payload)
				}
			}
		}
		frames = append(frames, f)
	}
	return frames
}

func newRecorderReporter(t *testing.T, command string) (*httptest.ResponseRecorder, *SSEReporter) {
	t.Helper()
	rec := httptest.NewRecorder()
	r, err := NewSSEReporter(rec, command, zap.NewNop())
	if err != nil {
		t.Fatalf("NewSSEReporter: %v", err)
	}
	return rec, r
}

func TestSSEReporter_CompletedEnvelope(t *testing.T) {
	rec, r := newRecorderReporter(t, "evaluate")

	r.Report(context.Background(), Event{Stage: StagePreparing, Message: "Preparing…"})
	r.Report(context.Background(), Event{Stage: StageStarting, Message: "Starting…"})
	r.Report(context.Background(), Event{
		Stage: StageCompleted,
		Data:  map[string]any{"feedback": []map[string]any{{"message": "Well done"}}},
	})

	if !rec.Flushed {
		t.Error("expected the response to be flushed")
	}

	frames := parseSSEFrames(t, rec.Body.String())
	if len(frames) != 3 {
		t.Fatalf("expected 3 frames (preparing, starting, completed), got %d: %q", len(frames), rec.Body.String())
	}
	if frames[0].event != "preparing" || frames[1].event != "starting" {
		t.Errorf("unexpected live frame events: %q, %q", frames[0].event, frames[1].event)
	}
	f := frames[2]
	if f.event != "completed" {
		t.Errorf("expected event 'completed', got %q", f.event)
	}
	if _, hasCommand := f.data["command"]; hasCommand {
		t.Errorf("terminal frame must not carry a command key: %v", f.data)
	}
	fb, ok := f.data["feedback"].([]any)
	if !ok || len(fb) != 1 {
		t.Fatalf("expected feedback array of 1, got %v", f.data["feedback"])
	}
	if fb[0].(map[string]any)["message"] != "Well done" {
		t.Errorf("feedback item not carried through: %v", fb[0])
	}
	steps, ok := f.data["steps"].([]any)
	if !ok || len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %v", f.data["steps"])
	}
	if steps[0].(map[string]any)["stage"] != "preparing" || steps[1].(map[string]any)["stage"] != "starting" {
		t.Errorf("unexpected step stages: %v", steps)
	}
	if steps[0].(map[string]any)["timestamp"] == "" {
		t.Errorf("expected step timestamp to be set")
	}
}

func TestSSEReporter_FailedEnvelope(t *testing.T) {
	rec, r := newRecorderReporter(t, "evaluate")

	r.Report(context.Background(), Event{Stage: StagePreparing, Message: "Preparing…"})
	r.Report(context.Background(), Event{
		Stage:   StageFailed,
		Error:   "worker send: context deadline exceeded",
		Message: "We couldn't evaluate your answer. Please try again.",
		ErrorInfo: &ErrorInfo{
			Title:   "Evaluation failed",
			Message: "We couldn't evaluate your answer. Please try again.",
			Code:    "INTERNAL_ERROR",
			Trace:   "worker send: context deadline exceeded",
		},
	})

	frames := parseSSEFrames(t, rec.Body.String())
	if len(frames) != 2 {
		t.Fatalf("expected 2 frames (preparing, failed), got %d", len(frames))
	}
	if frames[0].event != "preparing" {
		t.Errorf("expected first frame 'preparing', got %q", frames[0].event)
	}
	f := frames[1]
	if f.event != "failed" {
		t.Errorf("expected event 'failed', got %q", f.event)
	}
	if v, ok := f.data["feedback"]; !ok || v != nil {
		t.Errorf("expected feedback null, got %v (present=%v)", v, ok)
	}
	errObj, ok := f.data["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error to be an ErrorResponse object, got %T: %v", f.data["error"], f.data["error"])
	}
	if errObj["title"] != "Evaluation failed" {
		t.Errorf("error title not carried: %v", errObj["title"])
	}
	if errObj["message"] != "We couldn't evaluate your answer. Please try again." {
		t.Errorf("error message not carried: %v", errObj["message"])
	}
	if errObj["trace"] != "worker send: context deadline exceeded" {
		t.Errorf("error trace not carried: %v", errObj["trace"])
	}
	if _, hasMessage := f.data["message"]; hasMessage {
		t.Errorf("terminal frame must not carry a top-level message key: %v", f.data)
	}
	if steps, _ := f.data["steps"].([]any); len(steps) != 1 {
		t.Errorf("expected 1 step, got %v", f.data["steps"])
	}
}

func TestSSEReporter_PreviewUsesFeedbackEnvelope(t *testing.T) {
	rec, r := newRecorderReporter(t, "preview")
	r.Report(context.Background(), Event{
		Stage: StageCompleted,
		Data:  map[string]any{"feedback": []map[string]any{{"preSubmissionFeedback": map[string]any{}}}},
	})
	frames := parseSSEFrames(t, rec.Body.String())
	f := frames[0]
	if f.event != "completed" {
		t.Fatalf("expected 'completed', got %q", f.event)
	}
	if _, hasCommand := f.data["command"]; hasCommand {
		t.Errorf("terminal frame must not carry a command key: %v", f.data)
	}
	fb, ok := f.data["feedback"].([]any)
	if !ok || len(fb) != 1 {
		t.Fatalf("preview should use the feedback envelope, got %v", f.data["feedback"])
	}
	if _, ok := f.data["steps"].([]any); !ok {
		t.Errorf("steps should always be present as an array, got %v", f.data["steps"])
	}
}

func TestSSEReporter_ChatEnvelope_Completed(t *testing.T) {
	rec, r := newRecorderReporter(t, "chat")

	r.Report(context.Background(), Event{Stage: StageThinking, Message: "Searching your notes…"})
	r.Report(context.Background(), Event{
		Stage: StageCompleted,
		Data: map[string]any{
			"output":   map[string]any{"role": "ASSISTANT", "content": "Here you go"},
			"metadata": map[string]any{"model": "x"},
		},
	})

	frames := parseSSEFrames(t, rec.Body.String())
	if len(frames) != 2 || frames[0].event != "thinking" || frames[1].event != "completed" {
		t.Fatalf("expected [thinking, completed], got %q", rec.Body.String())
	}
	f := frames[1]
	if _, hasCommand := f.data["command"]; hasCommand {
		t.Errorf("terminal frame must not carry a command key: %v", f.data)
	}
	if _, hasFeedback := f.data["feedback"]; hasFeedback {
		t.Errorf("chat envelope must not carry a feedback key: %v", f.data)
	}
	out, ok := f.data["output"].(map[string]any)
	if !ok || out["content"] != "Here you go" {
		t.Fatalf("expected output object, got %v", f.data["output"])
	}
	if md, ok := f.data["metadata"].(map[string]any); !ok || md["model"] != "x" {
		t.Errorf("expected metadata carried, got %v", f.data["metadata"])
	}
	if steps, _ := f.data["steps"].([]any); len(steps) != 1 {
		t.Errorf("expected 1 step, got %v", f.data["steps"])
	}
}

func TestSSEReporter_ChatEnvelope_Failed(t *testing.T) {
	rec, r := newRecorderReporter(t, "chat")

	r.Report(context.Background(), Event{
		Stage:   StageFailed,
		Error:   "chat failed: worker exited",
		Message: "We couldn't generate a response. Please try again.",
		ErrorInfo: &ErrorInfo{
			Title:   "Chat failed",
			Message: "We couldn't generate a response. Please try again.",
			Trace:   "chat failed: worker exited",
		},
	})

	f := parseSSEFrames(t, rec.Body.String())[0]
	if f.event != "failed" {
		t.Fatalf("expected 'failed', got %q", f.event)
	}
	if v, ok := f.data["output"]; !ok || v != nil {
		t.Errorf("expected output null, got %v (present=%v)", v, ok)
	}
	errObj, ok := f.data["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error to be an ErrorResponse object, got %T: %v", f.data["error"], f.data["error"])
	}
	if errObj["title"] != "Chat failed" {
		t.Errorf("error title not carried: %v", errObj["title"])
	}
	if errObj["trace"] != "chat failed: worker exited" {
		t.Errorf("error trace not carried: %v", errObj["trace"])
	}
}

func TestSSEReporter_DedupLifecycleStagesOncePerRequest(t *testing.T) {
	rec, r := newRecorderReporter(t, "evaluate")
	// The per-case evaluation loop re-emits the shim's preparing/starting
	// markers once per case; only the first of each for the whole request
	// is kept. Worker-authored evaluating sub-steps are never collapsed.
	for _, s := range []Stage{StagePreparing, StagePreparing, StageStarting, StageStarting, StageEvaluating, StageEvaluating, StageStarting} {
		r.Report(context.Background(), Event{Stage: s})
	}
	r.Report(context.Background(), Event{Stage: StageCompleted, Data: map[string]any{"feedback": []map[string]any{}}})

	frames := parseSSEFrames(t, rec.Body.String())

	var liveEvents []string
	for _, f := range frames[:len(frames)-1] {
		liveEvents = append(liveEvents, f.event)
	}
	if strings.Join(liveEvents, ",") != "preparing,starting,evaluating,evaluating" {
		t.Errorf("expected live frames [preparing starting evaluating evaluating], got %v", liveEvents)
	}

	steps := frames[len(frames)-1].data["steps"].([]any)
	got := []string{}
	for _, s := range steps {
		got = append(got, s.(map[string]any)["stage"].(string))
	}
	want := []string{"preparing", "starting", "evaluating", "evaluating"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("expected steps %v, got %v", want, got)
	}
}

func TestSSEReporter_ProgressStepsNotDedupedAndTimestamped(t *testing.T) {
	rec, r := newRecorderReporter(t, "evaluate")
	r.Report(context.Background(), Event{Stage: StageProgress, Message: "same"})
	r.Report(context.Background(), Event{Stage: StageProgress, Message: "same"})
	r.Report(context.Background(), Event{Stage: StageProgress, Message: "same"})
	r.Report(context.Background(), Event{Stage: StageCompleted, Data: map[string]any{"feedback": []map[string]any{}}})

	frames := parseSSEFrames(t, rec.Body.String())
	if len(frames) != 4 {
		t.Fatalf("expected 4 frames (3 progress + completed), got %d: %q", len(frames), rec.Body.String())
	}
	for _, f := range frames[:3] {
		if f.event != "progress" {
			t.Errorf("expected a 'progress' live frame, got %q", f.event)
		}
	}

	steps := frames[3].data["steps"].([]any)
	if len(steps) != 3 {
		t.Fatalf("expected 3 progress steps, got %d", len(steps))
	}
	for _, s := range steps {
		ts, _ := s.(map[string]any)["timestamp"].(string)
		parsed, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil || parsed.IsZero() {
			t.Errorf("expected a non-zero RFC3339 timestamp, got %q (err=%v)", ts, err)
		}
	}
}

func TestSSEReporter_TerminalOnce(t *testing.T) {
	rec, r := newRecorderReporter(t, "evaluate")
	r.Report(context.Background(), Event{Stage: StageFailed, Message: "first"})
	r.Report(context.Background(), Event{Stage: StageFailed, Message: "second"})
	r.Report(context.Background(), Event{Stage: StageCompleted, Data: map[string]any{"feedback": []map[string]any{}}})

	frames := parseSSEFrames(t, rec.Body.String())
	if len(frames) != 1 {
		t.Fatalf("expected exactly 1 terminal frame, got %d", len(frames))
	}
	if frames[0].event != "failed" {
		t.Errorf("expected the first terminal event ('failed') to win, got %q", frames[0].event)
	}
	errObj, _ := frames[0].data["error"].(map[string]any)
	if errObj["message"] != "first" {
		t.Errorf("expected the first terminal event to win, got %v", errObj["message"])
	}
}

func TestSSEReporter_LateProgressAfterTerminalDropped(t *testing.T) {
	rec, r := newRecorderReporter(t, "evaluate")
	r.Report(context.Background(), Event{Stage: StageCompleted, Data: map[string]any{"feedback": []map[string]any{}}})
	before := rec.Body.String()

	r.Report(context.Background(), Event{Stage: StageProgress, Message: "too late"})
	r.Heartbeat()

	if rec.Body.String() != before {
		t.Errorf("expected no output after terminal frame, got extra: %q", strings.TrimPrefix(rec.Body.String(), before))
	}
}

func TestSSEReporter_Heartbeat(t *testing.T) {
	rec, r := newRecorderReporter(t, "evaluate")
	r.Report(context.Background(), Event{Stage: StagePreparing})
	r.Heartbeat()
	r.Heartbeat()
	if got := strings.Count(rec.Body.String(), ": ping\n\n"); got != 2 {
		t.Errorf("expected 2 heartbeat comments, got %d", got)
	}
	r.Report(context.Background(), Event{Stage: StageCompleted, Data: map[string]any{"feedback": []map[string]any{}}})
	r.Heartbeat()
	if got := strings.Count(rec.Body.String(), ": ping\n\n"); got != 2 {
		t.Errorf("expected heartbeat to be a no-op after terminal, got %d", got)
	}
}

func TestSSEReporter_ConcurrentReport(t *testing.T) {
	rec, r := newRecorderReporter(t, "evaluate")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.Report(context.Background(), Event{Stage: StageProgress, Message: "p"})
			r.Heartbeat()
		}()
	}
	wg.Wait()
	r.Report(context.Background(), Event{Stage: StageCompleted, Data: map[string]any{"feedback": []map[string]any{}}})

	// Every frame must be a well-formed event/data pair (parseSSEFrames
	// fails the test otherwise). Exactly one terminal frame, and it's last.
	frames := parseSSEFrames(t, rec.Body.String())
	terminals := 0
	for i, f := range frames {
		if f.event == "completed" || f.event == "failed" {
			terminals++
			if i != len(frames)-1 {
				t.Errorf("terminal frame at index %d is not last of %d", i, len(frames))
			}
		} else if f.event != "progress" {
			t.Errorf("unexpected intermediate frame event %q", f.event)
		}
	}
	if terminals != 1 {
		t.Fatalf("expected exactly 1 terminal frame, got %d: %q", terminals, rec.Body.String())
	}
}

type nonFlusherWriter struct{ h http.Header }

func (n nonFlusherWriter) Header() http.Header         { return n.h }
func (n nonFlusherWriter) Write(b []byte) (int, error) { return len(b), nil }
func (n nonFlusherWriter) WriteHeader(int)             {}

func TestNewSSEReporter_NonFlusher_Error(t *testing.T) {
	_, err := NewSSEReporter(nonFlusherWriter{h: http.Header{}}, "evaluate", zap.NewNop())
	if err == nil {
		t.Fatal("expected an error for a non-flushable writer")
	}
}

func TestSSEReporter_LiveFrameMatchesTerminalStep(t *testing.T) {
	rec, r := newRecorderReporter(t, "evaluate")

	r.Report(context.Background(), Event{
		Stage:   StageProgress,
		Message: "Parsing response and answer...",
		Data:    map[string]any{"step": float64(1), "of": float64(4)},
	})
	r.Report(context.Background(), Event{Stage: StageCompleted, Data: map[string]any{"feedback": []map[string]any{}}})

	frames := parseSSEFrames(t, rec.Body.String())
	if len(frames) != 2 {
		t.Fatalf("expected 2 frames, got %d: %q", len(frames), rec.Body.String())
	}

	live := frames[0]
	if live.event != "progress" {
		t.Fatalf("expected 'progress' live frame, got %q", live.event)
	}

	steps := frames[1].data["steps"].([]any)
	if len(steps) != 1 {
		t.Fatalf("expected 1 terminal step, got %d", len(steps))
	}

	// The live frame's data payload must be byte-identical to the matching
	// terminal steps[] element.
	wantJSON, _ := json.Marshal(steps[0])
	gotJSON, _ := json.Marshal(live.data)
	if string(wantJSON) != string(gotJSON) {
		t.Errorf("live frame data != terminal step:\n live: %s\n step: %s", gotJSON, wantJSON)
	}
}

// failingAfterNWriter is an http.Flusher whose Write starts returning an
// error after okWrites successful writes.
type failingAfterNWriter struct {
	h        http.Header
	okWrites int
	writes   int
	flushed  int
}

func (w *failingAfterNWriter) Header() http.Header { return w.h }
func (w *failingAfterNWriter) WriteHeader(int)     {}
func (w *failingAfterNWriter) Flush()              { w.flushed++ }
func (w *failingAfterNWriter) Write(b []byte) (int, error) {
	w.writes++
	if w.writes > w.okWrites {
		return 0, io.ErrClosedPipe
	}
	return len(b), nil
}

func TestSSEReporter_LiveFrameWriteErrorDoesNotTerminate(t *testing.T) {
	w := &failingAfterNWriter{h: http.Header{}, okWrites: 1}
	r, err := NewSSEReporter(w, "evaluate", zap.NewNop())
	if err != nil {
		t.Fatalf("NewSSEReporter: %v", err)
	}

	// First live frame writes OK; the second fails at the writer.
	r.Report(context.Background(), Event{Stage: StageProgress, Message: "one"})
	r.Report(context.Background(), Event{Stage: StageProgress, Message: "two"})

	if r.terminated {
		t.Fatal("a live-frame write error must not set terminated")
	}

	// The terminal frame is still attempted (Write is called again).
	writesBefore := w.writes
	r.Report(context.Background(), Event{Stage: StageCompleted, Data: map[string]any{"feedback": []map[string]any{}}})
	if w.writes == writesBefore {
		t.Error("expected the terminal frame to still attempt a write after a live-frame write error")
	}
	if !r.terminated {
		t.Error("expected terminated to be set once the terminal frame ran")
	}
}
