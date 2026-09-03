package progress

import (
	"context"
	"time"
)

// Reporter delivers progress events for a single evaluation request.
type Reporter interface {
	// Report emits a single event. Implementations MUST NOT return an
	// error to the caller and MUST apply their own bounded timeout —
	// progress delivery must never fail or slow down the evaluation.
	Report(ctx context.Context, evt Event)
}

type contextKey int

var reporterKey = contextKey(0)

// ContextWithReporter returns a copy of ctx carrying the given Reporter.
func ContextWithReporter(ctx context.Context, r Reporter) context.Context {
	return context.WithValue(ctx, reporterKey, r)
}

// FromContext returns the Reporter attached to ctx, or nil if none is
// attached. A nil Reporter is the expected, common case: most requests
// don't opt in to progress reporting.
func FromContext(ctx context.Context) Reporter {
	r, _ := ctx.Value(reporterKey).(Reporter)
	return r
}

// Emit is the call-site convenience for reporting a progress event. It is
// a silent no-op when no Reporter is attached to ctx, which is what makes
// progress reporting purely opt-in/additive.
func Emit(ctx context.Context, evt Event) {
	r := FromContext(ctx)
	if r == nil {
		return
	}

	evt.Timestamp = time.Now().UTC()
	r.Report(ctx, evt)
}
