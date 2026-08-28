package progress

import (
	"context"
	"encoding/json"
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

	r.Report(context.Background(), Event{Stage: StagePreparing, Message: "Preparing your evaluation…"})
	r.Report(context.Background(), Event{Stage: StageEvaluating, Message: "Evaluating your submission…"})
	r.Report(context.Background(), Event{
		Stage: StageCompleted,
		Data:  map[string]any{"feedback": []map[string]any{{"message": "Well done"}}},
	})

	if !rec.Flushed {
		t.Error("expected the response to be flushed")
	}

	frames := parseSSEFrames(t, rec.Body.String())
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d: %q", len(frames), rec.Body.String())
	}
	f := frames[0]
	if f.event != "completed" {
		t.Errorf("expected event 'completed', got %q", f.event)
	}
	if f.data["command"] != "evaluate" {
		t.Errorf("expected command 'evaluate', got %v", f.data["command"])
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
	if steps[0].(map[string]any)["stage"] != "preparing" || steps[1].(map[string]any)["stage"] != "evaluating" {
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
	})

	frames := parseSSEFrames(t, rec.Body.String())
	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(frames))
	}
	f := frames[0]
	if f.event != "failed" {
		t.Errorf("expected event 'failed', got %q", f.event)
	}
	if v, ok := f.data["feedback"]; !ok || v != nil {
		t.Errorf("expected feedback null, got %v (present=%v)", v, ok)
	}
	if f.data["error"] != "worker send: context deadline exceeded" {
		t.Errorf("raw error not carried: %v", f.data["error"])
	}
	if f.data["message"] != "We couldn't evaluate your answer. Please try again." {
		t.Errorf("user message not carried: %v", f.data["message"])
	}
	if steps, _ := f.data["steps"].([]any); len(steps) != 1 {
		t.Errorf("expected 1 step, got %v", f.data["steps"])
	}
}

func TestSSEReporter_PreviewCommandLabel(t *testing.T) {
	rec, r := newRecorderReporter(t, "preview")
	r.Report(context.Background(), Event{
		Stage: StageCompleted,
		Data:  map[string]any{"feedback": []map[string]any{{"preSubmissionFeedback": map[string]any{}}}},
	})
	frames := parseSSEFrames(t, rec.Body.String())
	if frames[0].data["command"] != "preview" {
		t.Errorf("expected command 'preview', got %v", frames[0].data["command"])
	}
}

func TestSSEReporter_DedupConsecutivePreparingEvaluating(t *testing.T) {
	rec, r := newRecorderReporter(t, "evaluate")
	for _, s := range []Stage{StagePreparing, StagePreparing, StageEvaluating, StageEvaluating, StagePreparing} {
		r.Report(context.Background(), Event{Stage: s})
	}
	r.Report(context.Background(), Event{Stage: StageCompleted, Data: map[string]any{"feedback": []map[string]any{}}})

	steps := parseSSEFrames(t, rec.Body.String())[0].data["steps"].([]any)
	got := []string{}
	for _, s := range steps {
		got = append(got, s.(map[string]any)["stage"].(string))
	}
	want := []string{"preparing", "evaluating", "preparing"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("expected steps %v, got %v", want, got)
	}
}

func TestSSEReporter_ProgressStepsNotDedupedAndTimestamped(t *testing.T) {
	rec, r := newRecorderReporter(t, "evaluate")
	r.Report(context.Background(), Event{Stage: StageProgress, Message: "same"})
	r.Report(context.Background(), Event{Stage: StageProgress, Message: "same"})
	r.Report(context.Background(), Event{Stage: StageCompleted, Data: map[string]any{"feedback": []map[string]any{}}})

	steps := parseSSEFrames(t, rec.Body.String())[0].data["steps"].([]any)
	if len(steps) != 2 {
		t.Fatalf("expected 2 progress steps, got %d", len(steps))
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
	if frames[0].data["message"] != "first" {
		t.Errorf("expected the first terminal event to win, got %v", frames[0].data["message"])
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

	frames := parseSSEFrames(t, rec.Body.String())
	if len(frames) != 1 || frames[0].event != "completed" {
		t.Fatalf("expected exactly 1 completed frame, got %d: %q", len(frames), rec.Body.String())
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
