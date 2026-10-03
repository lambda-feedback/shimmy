package progress

import (
	"context"
	"testing"
)

type panicReporter struct{ called bool }

func (p *panicReporter) Report(context.Context, Event) {
	p.called = true
	panic("boom")
}

func TestMultiReporter_FansOutInOrder(t *testing.T) {
	a := &recordingReporter{}
	b := &recordingReporter{}
	m := NewMultiReporter(a, b)

	m.Report(context.Background(), Event{Stage: StagePreparing})
	m.Report(context.Background(), Event{Stage: StageCompleted})

	for name, r := range map[string]*recordingReporter{"a": a, "b": b} {
		evts := r.recorded()
		if len(evts) != 2 || evts[0].Stage != StagePreparing || evts[1].Stage != StageCompleted {
			t.Errorf("reporter %s: expected both events in order, got %v", name, evts)
		}
	}
}

func TestMultiReporter_ChildPanicIsolated(t *testing.T) {
	p := &panicReporter{}
	b := &recordingReporter{}
	m := NewMultiReporter(p, b)

	// must not panic out to the caller
	m.Report(context.Background(), Event{Stage: StageEvaluating})

	if !p.called {
		t.Error("expected the panicking reporter to have been called")
	}
	if evts := b.recorded(); len(evts) != 1 || evts[0].Stage != StageEvaluating {
		t.Errorf("expected the second reporter to still receive the event, got %v", evts)
	}
}
