package progress

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

func newTestReporter(t *testing.T, url string, timeout time.Duration) *httpCallbackReporter {
	t.Helper()
	return newHTTPReporter(&http.Client{}, url, "corr-1", timeout, zap.NewNop()).(*httpCallbackReporter)
}

func TestHTTPCallbackReporter_Report_DeliversPayload(t *testing.T) {
	var mu sync.Mutex
	var received []payload

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p payload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Errorf("failed to decode payload: %v", err)
		}
		mu.Lock()
		received = append(received, p)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := newTestReporter(t, srv.URL, time.Second)
	r.Report(context.Background(), Event{Stage: StageEvaluating, Command: "eval"})

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 request, got %d", len(received))
	}
	if received[0].CorrelationID != "corr-1" {
		t.Errorf("expected correlationId %q, got %q", "corr-1", received[0].CorrelationID)
	}
	if received[0].Stage != StageEvaluating {
		t.Errorf("expected stage %q, got %q", StageEvaluating, received[0].Stage)
	}
}

func TestHTTPCallbackReporter_Report_TerminalEventDeliveredOnlyOnce(t *testing.T) {
	var mu sync.Mutex
	var count int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := newTestReporter(t, srv.URL, time.Second)

	// simulate both the supervisor and handler layers independently
	// detecting failure and trying to emit a terminal event
	r.Report(context.Background(), Event{Stage: StageFailed, Message: "boot failed"})
	r.Report(context.Background(), Event{Stage: StageFailed, Message: "handler backstop"})
	r.Report(context.Background(), Event{Stage: StageCompleted})

	mu.Lock()
	defer mu.Unlock()
	if count != 1 {
		t.Fatalf("expected exactly 1 terminal event delivered, got %d", count)
	}
}

func TestHTTPCallbackReporter_Report_NonTerminalEventsAllDelivered(t *testing.T) {
	var mu sync.Mutex
	var count int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := newTestReporter(t, srv.URL, time.Second)
	r.Report(context.Background(), Event{Stage: StagePreparing})
	r.Report(context.Background(), Event{Stage: StageEvaluating})

	mu.Lock()
	defer mu.Unlock()
	if count != 2 {
		t.Fatalf("expected 2 non-terminal events delivered, got %d", count)
	}
}

func TestHTTPCallbackReporter_Report_SlowReceiver_BoundedByTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := newTestReporter(t, srv.URL, 20*time.Millisecond)

	start := time.Now()
	r.Report(context.Background(), Event{Stage: StageEvaluating})
	elapsed := time.Since(start)

	if elapsed > 150*time.Millisecond {
		t.Errorf("expected Report to return promptly bounded by timeout, took %v", elapsed)
	}
}

func TestHTTPCallbackReporter_Report_UnreachableURL_DoesNotPanic(t *testing.T) {
	r := newTestReporter(t, "http://127.0.0.1:0", 50*time.Millisecond)
	r.Report(context.Background(), Event{Stage: StageEvaluating})
}
