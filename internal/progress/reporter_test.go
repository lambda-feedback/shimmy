package progress

import (
	"context"
	"sync"
	"testing"
)

// recordingReporter is a test double shared across this package's test
// files. It's safe for concurrent use since sidecar_test.go exercises it
// from the sidecar's detached relay goroutine as well as the test
// goroutine polling for results.
type recordingReporter struct {
	mu     sync.Mutex
	events []Event
}

func (r *recordingReporter) Report(_ context.Context, evt Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, evt)
}

// recorded returns a snapshot of the events received so far.
func (r *recordingReporter) recorded() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Event(nil), r.events...)
}

func TestEmit_NoReporterInContext_NoOp(t *testing.T) {
	// must not panic, must not do anything observable
	Emit(context.Background(), Event{Stage: StageEvaluating})
}

func TestEmit_WithReporter_DeliversEventAndSetsTimestamp(t *testing.T) {
	r := &recordingReporter{}
	ctx := ContextWithReporter(context.Background(), r)

	Emit(ctx, Event{Stage: StagePreparing, Command: "eval"})

	events := r.recorded()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	evt := events[0]
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
