package progress

import (
	"context"
	"testing"
)

type recordingReporter struct {
	events []Event
}

func (r *recordingReporter) Report(_ context.Context, evt Event) {
	r.events = append(r.events, evt)
}

func TestEmit_NoReporterInContext_NoOp(t *testing.T) {
	// must not panic, must not do anything observable
	Emit(context.Background(), Event{Stage: StageEvaluating})
}

func TestEmit_WithReporter_DeliversEventAndSetsTimestamp(t *testing.T) {
	r := &recordingReporter{}
	ctx := ContextWithReporter(context.Background(), r)

	Emit(ctx, Event{Stage: StagePreparing, Command: "eval"})

	if len(r.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(r.events))
	}
	evt := r.events[0]
	if evt.Stage != StagePreparing {
		t.Errorf("expected stage %q, got %q", StagePreparing, evt.Stage)
	}
	if evt.Command != "eval" {
		t.Errorf("expected command %q, got %q", "eval", evt.Command)
	}
	if evt.Timestamp.IsZero() {
		t.Errorf("expected Emit to set a non-zero timestamp")
	}
}

func TestFromContext_NoReporter_ReturnsNil(t *testing.T) {
	if r := FromContext(context.Background()); r != nil {
		t.Errorf("expected nil reporter, got %v", r)
	}
}
