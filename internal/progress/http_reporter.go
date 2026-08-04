package progress

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// payload is the JSON body POSTed to the callback URL for each event.
type payload struct {
	CorrelationID string         `json:"correlationId"`
	Stage         Stage          `json:"stage"`
	Command       string         `json:"command,omitempty"`
	Message       string         `json:"message,omitempty"`
	Error         string         `json:"error,omitempty"`
	Data          map[string]any `json:"data,omitempty"`
	Timestamp     time.Time      `json:"timestamp"`
}

// httpCallbackReporter delivers progress events as outbound HTTP POST
// requests to a caller-supplied URL.
type httpCallbackReporter struct {
	client        *http.Client
	url           string
	correlationID string
	timeout       time.Duration
	log           *zap.Logger

	terminalOnce sync.Once
}

var _ Reporter = (*httpCallbackReporter)(nil)

func newHTTPReporter(
	client *http.Client,
	url string,
	correlationID string,
	timeout time.Duration,
	log *zap.Logger,
) Reporter {
	return &httpCallbackReporter{
		client:        client,
		url:           url,
		correlationID: correlationID,
		timeout:       timeout,
		log:           log,
	}
}

// Report POSTs evt to the configured callback URL. Delivery is best-effort:
// any error (invalid payload, dial failure, timeout, non-2xx response) is
// logged and swallowed — it must never fail or slow down the evaluation
// beyond the configured timeout. At most one terminal event (StageFailed
// or StageCompleted) is delivered per reporter instance, since both
// the supervisor and handler layers can independently detect failure.
func (r *httpCallbackReporter) Report(ctx context.Context, evt Event) {
	if evt.Stage.terminal() {
		sent := false
		r.terminalOnce.Do(func() {
			r.send(ctx, evt)
			sent = true
		})
		if !sent {
			r.log.Debug("dropping duplicate terminal progress event", zap.String("stage", string(evt.Stage)))
		}
		return
	}

	r.send(ctx, evt)
}

func (r *httpCallbackReporter) send(ctx context.Context, evt Event) {
	body, err := json.Marshal(payload{
		CorrelationID: r.correlationID,
		Stage:         evt.Stage,
		Command:       evt.Command,
		Message:       evt.Message,
		Error:         evt.Error,
		Data:          evt.Data,
		Timestamp:     evt.Timestamp,
	})
	if err != nil {
		r.log.Warn("failed to marshal progress event", zap.String("stage", string(evt.Stage)), zap.Error(err))
		return
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, bytes.NewReader(body))
	if err != nil {
		r.log.Warn("failed to build progress callback request", zap.String("stage", string(evt.Stage)), zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		r.log.Warn("progress callback delivery failed", zap.String("stage", string(evt.Stage)), zap.Error(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		r.log.Warn("progress callback returned non-2xx status",
			zap.String("stage", string(evt.Stage)),
			zap.Int("status", resp.StatusCode),
		)
	}
}
