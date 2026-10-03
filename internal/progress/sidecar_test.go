package progress

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

func newTestSidecar(t *testing.T, cfg SidecarConfig) *Sidecar {
	t.Helper()
	s, err := NewSidecar(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to start sidecar: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func postSidecar(t *testing.T, s *Sidecar, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(s.URL(), "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("failed to POST to sidecar: %v", err)
	}
	defer resp.Body.Close()
	return resp
}

// waitForEvents polls until r has at least n events or the timeout expires,
// since the sidecar relays events in a detached goroutine.
func waitForEvents(t *testing.T, r *recordingReporter, n int) []Event {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if events := r.recorded(); len(events) >= n {
			return events
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d events, got %d", n, len(r.recorded()))
	return nil
}

func TestSidecar_Accept_RelaysEventThroughBoundReporter(t *testing.T) {
	s := newTestSidecar(t, SidecarConfig{})
	r := &recordingReporter{}
	s.Bind("eval", r)

	resp := postSidecar(t, s, `{"message":"checking correctness…","data":{"step":2}}`)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	events := waitForEvents(t, r, 1)
	evt := events[0]
	if evt.Stage != StageEvaluating {
		t.Errorf("expected stage %q, got %q", StageEvaluating, evt.Stage)
	}
	if evt.Command != "eval" {
		t.Errorf("expected command %q, got %q", "eval", evt.Command)
	}
	if evt.Message != "checking correctness…" {
		t.Errorf("unexpected message %q", evt.Message)
	}
	if evt.Data["step"] != float64(2) {
		t.Errorf("expected data.step=2, got %v", evt.Data["step"])
	}
}

func TestSidecar_IgnoresWorkerSuppliedStage(t *testing.T) {
	s := newTestSidecar(t, SidecarConfig{})
	r := &recordingReporter{}
	s.Bind("eval", r)

	resp := postSidecar(t, s, `{"message":"trying to spoof","stage":"completed"}`)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	events := waitForEvents(t, r, 1)
	if events[0].Stage != StageEvaluating {
		t.Errorf("worker-supplied stage must be ignored, got %q", events[0].Stage)
	}
}

func TestSidecar_StageFollowsBoundCommand(t *testing.T) {
	cases := map[string]Stage{
		"eval":        StageEvaluating,
		"preview":     StageEvaluating,
		"chat":        StageThinking,
		"chat/health": StageThinking,
	}
	for command, wantStage := range cases {
		t.Run(command, func(t *testing.T) {
			s := newTestSidecar(t, SidecarConfig{})
			r := &recordingReporter{}
			s.Bind(command, r)

			resp := postSidecar(t, s, `{"message":"working…"}`)
			if resp.StatusCode != http.StatusAccepted {
				t.Fatalf("expected 202, got %d", resp.StatusCode)
			}

			evt := waitForEvents(t, r, 1)[0]
			if evt.Stage != wantStage {
				t.Errorf("command %q: expected stage %q, got %q", command, wantStage, evt.Stage)
			}
		})
	}
}

func TestSidecar_RejectsEmptyMessage(t *testing.T) {
	s := newTestSidecar(t, SidecarConfig{})
	s.Bind("eval", &recordingReporter{})

	resp := postSidecar(t, s, `{"message":""}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSidecar_RejectsMalformedJSON(t *testing.T) {
	s := newTestSidecar(t, SidecarConfig{})
	s.Bind("eval", &recordingReporter{})

	resp := postSidecar(t, s, `not json`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSidecar_RejectsOversizedBody(t *testing.T) {
	s := newTestSidecar(t, SidecarConfig{MaxBodyBytes: 16})
	s.Bind("eval", &recordingReporter{})

	body := `{"message":"` + strings.Repeat("x", 100) + `"}`
	resp := postSidecar(t, s, body)
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", resp.StatusCode)
	}
}

func TestSidecar_RateLimit_MaxEventsPerSpan(t *testing.T) {
	// MinEventInterval is small (not disabled - 0 means "use the default")
	// and slept past between POSTs, so only MaxEventsPerSpan is under test.
	s := newTestSidecar(t, SidecarConfig{MaxEventsPerSpan: 1, MinEventInterval: time.Millisecond})
	r := &recordingReporter{}
	s.Bind("eval", r)

	first := postSidecar(t, s, `{"message":"one"}`)
	if first.StatusCode != http.StatusAccepted {
		t.Fatalf("expected first event accepted (202), got %d", first.StatusCode)
	}

	time.Sleep(5 * time.Millisecond)

	second := postSidecar(t, s, `{"message":"two"}`)
	if second.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected second event rate limited (429), got %d", second.StatusCode)
	}
}

func TestSidecar_RateLimit_MinEventInterval(t *testing.T) {
	// BurstSize disabled so the very first event is already subject to
	// interval spacing, isolating what this test exercises.
	s := newTestSidecar(t, SidecarConfig{MaxEventsPerSpan: 100, BurstSize: -1, MinEventInterval: time.Hour})
	r := &recordingReporter{}
	s.Bind("eval", r)

	first := postSidecar(t, s, `{"message":"one"}`)
	if first.StatusCode != http.StatusAccepted {
		t.Fatalf("expected first event accepted (202), got %d", first.StatusCode)
	}

	second := postSidecar(t, s, `{"message":"two"}`)
	if second.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected second event rate limited (429) by min interval, got %d", second.StatusCode)
	}
}

func TestSidecar_Burst_AllowsCloselySpacedEventsWithinBurst(t *testing.T) {
	// A large MinEventInterval would reject any second event immediately -
	// unless it falls within the burst allowance, which is what this
	// exercises: events 2 and 3 land inside BurstSize and must be accepted
	// even though far less than MinEventInterval separates them.
	s := newTestSidecar(t, SidecarConfig{MaxEventsPerSpan: 100, BurstSize: 3, MinEventInterval: time.Hour})
	r := &recordingReporter{}
	s.Bind("eval", r)

	for i, msg := range []string{"one", "two", "three"} {
		resp := postSidecar(t, s, `{"message":"`+msg+`"}`)
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("expected burst event %d accepted (202), got %d", i+1, resp.StatusCode)
		}
	}
}

func TestSidecar_Burst_ThenEnforcesMinEventInterval(t *testing.T) {
	s := newTestSidecar(t, SidecarConfig{MaxEventsPerSpan: 100, BurstSize: 2, MinEventInterval: time.Hour})
	r := &recordingReporter{}
	s.Bind("eval", r)

	for i, msg := range []string{"one", "two"} {
		resp := postSidecar(t, s, `{"message":"`+msg+`"}`)
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("expected burst event %d accepted (202), got %d", i+1, resp.StatusCode)
		}
	}

	third := postSidecar(t, s, `{"message":"three"}`)
	if third.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected event past burst allowance rate limited (429), got %d", third.StatusCode)
	}
}

func TestSidecar_Bind_ResetsRateLimitState(t *testing.T) {
	s := newTestSidecar(t, SidecarConfig{MaxEventsPerSpan: 1, MinEventInterval: time.Millisecond})
	r1 := &recordingReporter{}
	s.Bind("eval", r1)

	postSidecar(t, s, `{"message":"one"}`)
	exhausted := postSidecar(t, s, `{"message":"two"}`)
	if exhausted.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected span to be exhausted (429), got %d", exhausted.StatusCode)
	}

	r2 := &recordingReporter{}
	s.Bind("eval", r2)

	resp := postSidecar(t, s, `{"message":"fresh span"}`)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected fresh span to accept after re-Bind (202), got %d", resp.StatusCode)
	}
}

func TestSidecar_Unbound_Returns503(t *testing.T) {
	s := newTestSidecar(t, SidecarConfig{})

	resp := postSidecar(t, s, `{"message":"nobody home"}`)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 with no bound reporter, got %d", resp.StatusCode)
	}
}

func TestSidecar_Unbind_Returns503(t *testing.T) {
	s := newTestSidecar(t, SidecarConfig{})
	s.Bind("eval", &recordingReporter{})
	s.Unbind()

	resp := postSidecar(t, s, `{"message":"straggler"}`)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 after Unbind, got %d", resp.StatusCode)
	}
}
