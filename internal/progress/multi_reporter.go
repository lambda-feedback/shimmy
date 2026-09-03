package progress

import "context"

// multiReporter fans a single event out to several reporters. It's used
// when a request both opens an SSE stream and supplies a callbackUrl:
// each underlying reporter keeps its own terminal-once guard, so the
// fan-out needs no extra state.
type multiReporter struct {
	reporters []Reporter
}

var _ Reporter = (*multiReporter)(nil)

// NewMultiReporter returns a Reporter that delivers each event to every
// reporter in rs, in order. A reporter that panics or blocks must not
// prevent the others from receiving the event, nor propagate out to the
// evaluation goroutine.
func NewMultiReporter(rs ...Reporter) Reporter {
	return &multiReporter{reporters: rs}
}

func (m *multiReporter) Report(ctx context.Context, evt Event) {
	for _, r := range m.reporters {
		func() {
			defer func() { _ = recover() }()
			r.Report(ctx, evt)
		}()
	}
}
