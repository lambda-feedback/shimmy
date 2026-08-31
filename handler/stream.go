package handler

import (
	"context"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/lambda-feedback/shimmy/internal/progress"
)

// streamProgress runs a request whose progress is streamed back on the
// response as Server-Sent Events. It commits a 200 + event-stream headers
// immediately, wires an SSE reporter (fanned out to a callbackURL reporter
// too, if callbackURL is set) into ctx, keeps the connection alive with
// heartbeats while run executes, then emits exactly one terminal frame
// (completed | failed) built from run's result. Because the status is
// already committed, every outcome of run — including an internal error —
// becomes a "failed" frame, never an HTTP error.
//
// cmdLabel selects the terminal envelope shape ("evaluate"/"preview" ->
// feedback[]; "chat" -> output/metadata) and is the frame's "command".
// command is the µEd command string carried on the emitted progress
// events. doneMessage is the human-facing text on the terminal completed
// event.
//
// run returns the terminal event's Data payload (e.g. {"feedback": …} or
// {"output": …, "metadata": …}) on success, or a *terminalError. It must
// not write to w or emit progress events itself.
func (h *MuEdHandler) streamProgress(
	ctx context.Context,
	w http.ResponseWriter,
	cmdLabel string,
	command string,
	doneMessage string,
	version string,
	callbackURL string,
	requestID string,
	run func(ctx context.Context) (map[string]any, *terminalError),
) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set(muEdVersionHeader, version)
	w.WriteHeader(http.StatusOK)
	w.(http.Flusher).Flush()

	sseReporter, err := progress.NewSSEReporter(w, cmdLabel, h.log)
	if err != nil {
		// Guarded against by the caller; don't panic if it slips through.
		h.log.Error("failed to create SSE reporter", zap.Error(err))
		return
	}

	var reporter progress.Reporter = sseReporter
	if callbackURL != "" {
		cbReporter, cbErr := h.progressFactory.NewReporter(callbackURL, requestID)
		if cbErr != nil {
			h.log.Warn("invalid callbackUrl, disabling callback delivery", zap.Error(cbErr))
		} else if cbReporter != nil {
			reporter = progress.NewMultiReporter(sseReporter, cbReporter)
		}
	}
	ctx = progress.ContextWithReporter(ctx, reporter)

	done := make(chan struct{})
	var hbWG sync.WaitGroup
	if secs := h.config.Progress.Stream.HeartbeatSeconds; secs > 0 {
		hbWG.Add(1)
		go func() {
			defer hbWG.Done()
			ticker := time.NewTicker(time.Duration(secs) * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ctx.Done():
					return
				case <-ticker.C:
					sseReporter.Heartbeat()
				}
			}
		}()
	}

	data, termErr := run(ctx)
	if termErr != nil {
		progress.Emit(ctx, progress.Event{
			Stage:   progress.StageFailed,
			Command: command,
			Message: termErr.userMessage,
			Error:   termErr.rawError,
		})
	} else {
		progress.Emit(ctx, progress.Event{
			Stage:   progress.StageCompleted,
			Command: command,
			Message: doneMessage,
			Data:    data,
		})
	}

	close(done)
	hbWG.Wait()
}
