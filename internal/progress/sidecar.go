package progress

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	defaultSidecarMaxBodyBytes      int64 = 16 * 1024
	defaultSidecarMaxEventsPerSpan        = 50
	defaultSidecarBurstSize               = 5
	defaultSidecarMinEventInterval        = 10 * time.Millisecond
	defaultSidecarUnbindGracePeriod       = 250 * time.Millisecond
)

// SidecarConfig bounds abuse of the worker-authored progress side-channel.
// Since EVAL_PROGRESS_URL is reachable by arbitrary (and, under sandboxing,
// untrusted) worker code, delivery to the real callbackUrl must stay bounded
// regardless of how the worker behaves.
type SidecarConfig struct {
	// MaxBodyBytes caps the size of a single progress event POST body.
	// If unset (<= 0), defaultSidecarMaxBodyBytes is used.
	MaxBodyBytes int64 `conf:"max_body_bytes"`

	// MaxEventsPerSpan caps how many progress events a single evaluation
	// span (the window between Bind and the next Bind/Unbind) may relay.
	// If unset (<= 0), defaultSidecarMaxEventsPerSpan is used.
	MaxEventsPerSpan int `conf:"max_events_per_span"`

	// BurstSize is how many events at the start of a span are exempt from
	// MinEventInterval spacing, so a handful of legitimate back-to-back
	// checkpoints (e.g. a fast evaluation reporting progress at several
	// points microseconds to a few ms apart) aren't rate-limited just
	// because they arrive faster than any fixed spacing could accommodate.
	// MinEventInterval spacing only applies once the burst is used up.
	// Still bounded by MaxEventsPerSpan. If unset (== 0),
	// defaultSidecarBurstSize is used; a negative value explicitly
	// disables the burst allowance (spacing applies from the first event).
	BurstSize int `conf:"burst_size"`

	// MinEventInterval enforces a minimum spacing between accepted events
	// once a span's BurstSize allowance is used up. If unset (<= 0),
	// defaultSidecarMinEventInterval is used.
	MinEventInterval time.Duration `conf:"min_event_interval"`

	// UnbindGracePeriod delays detaching the bound reporter after a span
	// ends, so a worker-authored progress POST that was already in flight
	// (e.g. dispatched fire-and-forget just before the worker returned its
	// result) still has a window to arrive and be relayed, instead of
	// racing the RPC response back to shim. If unset (<= 0),
	// defaultSidecarUnbindGracePeriod is used.
	UnbindGracePeriod time.Duration `conf:"unbind_grace_period"`
}

func (c SidecarConfig) withDefaults() SidecarConfig {
	if c.MaxBodyBytes <= 0 {
		c.MaxBodyBytes = defaultSidecarMaxBodyBytes
	}
	if c.MaxEventsPerSpan <= 0 {
		c.MaxEventsPerSpan = defaultSidecarMaxEventsPerSpan
	}
	if c.BurstSize < 0 {
		c.BurstSize = 0
	} else if c.BurstSize == 0 {
		c.BurstSize = defaultSidecarBurstSize
	}
	if c.MinEventInterval <= 0 {
		c.MinEventInterval = defaultSidecarMinEventInterval
	}
	if c.UnbindGracePeriod <= 0 {
		c.UnbindGracePeriod = defaultSidecarUnbindGracePeriod
	}
	return c
}

// sidecarPayload is the JSON body a worker POSTs to report a custom
// progress event. There is deliberately no "stage" field: a worker can
// never choose its own stage. The sidecar assigns one from the command in
// flight (see stageForCommand). Unknown fields (including a "stage" a
// worker might send anyway) are silently ignored by json.Decode, never
// merged in.
type sidecarPayload struct {
	Message string         `json:"message"`
	Data    map[string]any `json:"data,omitempty"`
}

// stageForCommand maps the command bound to the sidecar onto the stage a
// worker-authored sub-step is relayed under: chat commands report
// "thinking", everything else (eval, preview, …) reports "evaluating".
func stageForCommand(command string) Stage {
	switch command {
	case "chat", "chat/health":
		return StageThinking
	default:
		return StageEvaluating
	}
}

// Sidecar is a loopback-only HTTP listener that accepts worker-authored
// progress events and relays them, best-effort, through whichever Reporter
// is currently Bind-ed to it. It is the counterpart, on the inbound side,
// to the outbound delivery in http_reporter.go: since it only ever binds
// to 127.0.0.1, it needs no SSRF guarding, but it does need its own abuse
// limits, since the worker producing events may be untrusted.
//
// Its lifetime differs by adapter: for a persistent RPC worker, one Sidecar
// lives for the worker's whole lifetime and is Bind/Unbind-ed around each
// request; for the transient file interface, one Sidecar is created and
// Closed per request.
type Sidecar struct {
	cfg SidecarConfig
	log *zap.Logger

	listener net.Listener
	server   *http.Server

	mu         sync.Mutex
	command    string
	reporter   Reporter
	count      int
	lastSent   time.Time
	generation uint64
}

// NewSidecar starts a loopback HTTP listener on an OS-assigned port.
func NewSidecar(cfg SidecarConfig, log *zap.Logger) (*Sidecar, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start progress sidecar listener: %w", err)
	}

	s := &Sidecar{
		cfg:      cfg.withDefaults(),
		log:      log.Named("progress_sidecar"),
		listener: ln,
	}

	s.server = &http.Server{
		Handler:           http.HandlerFunc(s.handle),
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       2 * time.Second,
		WriteTimeout:      2 * time.Second,
	}

	go func() {
		if err := s.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.Warn("progress sidecar listener stopped unexpectedly", zap.Error(err))
		}
	}()

	return s, nil
}

// URL returns the sidecar's loopback address, suitable for EVAL_PROGRESS_URL.
func (s *Sidecar) URL() string {
	return "http://" + s.listener.Addr().String()
}

// Bind associates command/reporter with the sidecar for the duration of one
// evaluation span, resetting rate-limit state so a fresh span isn't
// poisoned by the previous request's usage. Call at the start of an
// adapter's Send. A nil reporter behaves like Unbind.
func (s *Sidecar) Bind(command string, reporter Reporter) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.generation++
	s.command = command
	s.reporter = reporter
	s.count = 0
	s.lastSent = time.Time{}
}

// Unbind detaches the current reporter immediately, so any subsequent POST
// (e.g. a straggler arriving after the bound request has already returned)
// is rejected with 503 rather than misattributed to a future, unrelated
// request.
func (s *Sidecar) Unbind() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.generation++
	s.command = ""
	s.reporter = nil
}

// UnbindAfterGrace schedules the detach for after cfg.UnbindGracePeriod
// instead of doing it immediately, without blocking the caller. This gives
// a worker-authored progress POST dispatched fire-and-forget just before
// the RPC response reached shim a window to still arrive and be relayed,
// rather than losing the race against Unbind and being rejected with 503.
//
// If a new span is Bind-ed (or explicitly Unbind-ed) before the grace
// period elapses, this is a no-op: the generation captured at schedule time
// will no longer match, so the stale detach never fires and never clobbers
// the newer span.
func (s *Sidecar) UnbindAfterGrace() {
	s.mu.Lock()
	gen := s.generation
	grace := s.cfg.UnbindGracePeriod
	s.mu.Unlock()

	if grace <= 0 {
		s.Unbind()
		return
	}

	time.AfterFunc(grace, func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		if s.generation != gen {
			return
		}

		s.generation++
		s.command = ""
		s.reporter = nil
	})
}

// Close shuts down the sidecar's listener. It does not wait for any
// in-flight relayed events (those run detached from the listener, see
// handle) — consistent with progress delivery never blocking shutdown.
func (s *Sidecar) Close() error {
	return s.server.Close()
}

func (s *Sidecar) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxBodyBytes)

	var body sidecarPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(body.Message) == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	command, reporter, status := s.accept()
	if status != 0 {
		w.WriteHeader(status)
		return
	}

	w.WriteHeader(http.StatusAccepted)

	evt := Event{
		Stage:   stageForCommand(command),
		Command: command,
		Message: body.Message,
		Data:    body.Data,
	}

	// Relay detached from the inbound request: the worker's POST must
	// never be held open for the outbound callbackUrl delivery, which has
	// its own bounded timeout inside Report.
	go reporter.Report(context.Background(), evt)
}

// accept reports whether a new event may be relayed right now, applying
// the bound reporter check and the abuse limits. status is 0 on success,
// or the HTTP status to reject the request with otherwise.
func (s *Sidecar) accept() (command string, reporter Reporter, status int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.reporter == nil {
		return "", nil, http.StatusServiceUnavailable
	}

	now := time.Now()
	if s.count >= s.cfg.MaxEventsPerSpan {
		return "", nil, http.StatusTooManyRequests
	}
	if s.count >= s.cfg.BurstSize && !s.lastSent.IsZero() && now.Sub(s.lastSent) < s.cfg.MinEventInterval {
		return "", nil, http.StatusTooManyRequests
	}

	s.count++
	s.lastSent = now

	return s.command, s.reporter, 0
}
