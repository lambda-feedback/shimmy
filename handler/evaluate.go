package handler

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/lambda-feedback/shimmy/config"
	"github.com/lambda-feedback/shimmy/internal/progress"
	"github.com/lambda-feedback/shimmy/internal/server"
	"github.com/lambda-feedback/shimmy/runtime"
)

const muEdVersionHeader = "X-Api-Version"

// muEdRequestIDHeader is the µEd spec's request-tracing header (see
// https://mued.org/spec, X-Request-Id parameter). It's echoed back on every
// response, generating one if the caller didn't supply it, and progress
// events reuse the resolved value as their correlation key.
const muEdRequestIDHeader = "X-Request-Id"

// resolveRequestID returns the caller-supplied X-Request-Id, or generates
// one if absent, so every request is traceable and correlatable even when
// the caller doesn't participate in tracing itself.
func resolveRequestID(r *http.Request) string {
	if id := r.Header.Get(muEdRequestIDHeader); id != "" {
		return id
	}
	return generateRequestID()
}

func generateRequestID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand.Read on a real OS essentially never fails; fall back
		// to a timestamp-based id rather than leaving the request untraceable.
		return fmt.Sprintf("req-%08x", time.Now().UnixNano())
	}
	return fmt.Sprintf("req-%x", b)
}

type MuEdHandlerParams struct {
	fx.In

	Handler             runtime.Handler
	Runtime             runtime.Runtime
	Registry            *runtime.MuEdRegistry
	Config              config.Config
	Log                 *zap.Logger
	ProgressFactory     progress.Factory
	StreamingCapability StreamingCapability

	// Spec is the µEd OpenAPI spec, used to validate the SSE terminal
	// frame payload (the streamed analogue of the buffered path's
	// response validation). Optional: absent under AWS Lambda, which
	// cannot stream anyway.
	Spec *openapi3.T `optional:"true"`
}

type MuEdHandler struct {
	handler          runtime.Handler
	runtime          runtime.Runtime
	registry         *runtime.MuEdRegistry
	config           config.Config
	log              *zap.Logger
	progressFactory  progress.Factory
	streamingCapable bool
	spec             *openapi3.T
}

func NewMuEdHandler(params MuEdHandlerParams) *MuEdHandler {
	return &MuEdHandler{
		handler:          params.Handler,
		runtime:          params.Runtime,
		registry:         params.Registry,
		config:           params.Config,
		log:              params.Log,
		progressFactory:  params.ProgressFactory,
		streamingCapable: params.StreamingCapability.Enabled,
		spec:             params.Spec,
	}
}

// muEdRegistry returns the handler's version registry, falling back to the
// process-wide default when none was injected (e.g. in unit tests).
func (h *MuEdHandler) muEdRegistry() *runtime.MuEdRegistry {
	if h.registry != nil {
		return h.registry
	}
	return runtime.DefaultMuEdRegistry()
}

// streamingEnabled reports whether this deployment can and should stream
// SSE progress: a streaming-capable environment with streaming turned on
// in config. It gates both the runtime decision to stream a response and
// the capability shimmy advertises on its health endpoints.
func (h *MuEdHandler) streamingEnabled() bool {
	return h.streamingCapable && h.config.Progress.Stream.Enabled
}

func writeJSONError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": msg}}) //nolint:errcheck
}

// checkMuEdVersion validates the X-Api-Version request header and resolves it to
// a concrete version adapter. Returns (resolvedVersion, adapter, true) on
// success, or writes a 406 and returns ("", nil, false).
func (h *MuEdHandler) checkMuEdVersion(w http.ResponseWriter, r *http.Request) (string, runtime.MuEdAdapter, bool) {
	reg := h.muEdRegistry()
	requested := r.Header.Get(muEdVersionHeader)
	if requested != "" && !reg.Supports(requested) {
		body, _ := json.Marshal(map[string]any{
			"title": "API version not supported",
			"message": fmt.Sprintf(
				"The requested API version '%s' is not supported. Supported versions are: %v.",
				requested, reg.Versions(),
			),
			"code": "VERSION_NOT_SUPPORTED",
			"details": map[string]any{
				"requestedVersion":  requested,
				"supportedVersions": reg.Versions(),
			},
		})
		w.Header().Set(muEdVersionHeader, reg.Resolve(requested))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write(body) //nolint:errcheck
		return "", nil, false
	}
	version := reg.Resolve(requested)
	return version, reg.Adapter(version), true
}

// writeMuEdError writes a structured muEd JSON error response with X-Api-Version header.
func (h *MuEdHandler) writeMuEdError(w http.ResponseWriter, version string, statusCode int, code, title, message string, details map[string]any) {
	body, _ := json.Marshal(map[string]any{
		"title":   title,
		"message": message,
		"code":    code,
		"details": details,
	})
	w.Header().Set(muEdVersionHeader, version)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(body) //nolint:errcheck
}

func (h *MuEdHandler) checkAuth(w http.ResponseWriter, r *http.Request) bool {
	if h.config.Auth.Key != "" && r.Header.Get("api-key") != h.config.Auth.Key {
		h.log.Debug("unauthorized request", zap.String("path", r.URL.Path))
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

// ServeEvaluate handles POST /evaluate.
func (h *MuEdHandler) ServeEvaluate(w http.ResponseWriter, r *http.Request) {
	requestID := resolveRequestID(r)
	w.Header().Set(muEdRequestIDHeader, requestID)

	if !h.checkAuth(w, r) {
		return
	}

	version, adapter, ok := h.checkMuEdVersion(w, r)
	if !ok {
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeMuEdError(w, version, http.StatusBadRequest, "VALIDATION_ERROR", "Bad request", "failed to read body", nil)
		return
	}

	legacyBody, command, err := adapter.DecodeEvaluate(body)
	if err != nil {
		h.writeMuEdError(w, version, http.StatusBadRequest, "VALIDATION_ERROR", "Bad request", err.Error(), nil)
		return
	}

	legacyBodyBytes, err := json.Marshal(legacyBody)
	if err != nil {
		h.writeMuEdError(w, version, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", "failed to build request", nil)
		return
	}

	header := http.Header{}
	header.Set("Command", string(command))

	req := runtime.Request{
		Path:   r.URL.Path,
		Method: http.MethodPost,
		Body:   legacyBodyBytes,
		Header: header,
	}

	// The adapter's DecodeEvaluate deliberately does not surface transport
	// concerns like callbackUrl, so pull it from the raw body here. A parse
	// failure is impossible in practice — DecodeEvaluate already parsed the
	// same bytes — so a miss just means no out-of-band progress delivery.
	var callbackURL string
	var muEdReq runtime.MuEdEvaluateRequest
	if json.Unmarshal(body, &muEdReq) == nil && muEdReq.CallbackUrl != nil {
		callbackURL = *muEdReq.CallbackUrl
	}

	streaming := h.streamingEnabled() && acceptsEventStream(r)
	if streaming {
		if _, ok := w.(http.Flusher); !ok {
			h.log.Warn("response writer is not a flusher; serving buffered response")
			streaming = false
		}
	}

	ctx := r.Context()

	if streaming {
		h.serveEvaluateStream(ctx, w, req, adapter, command, version, callbackURL, requestID)
		return
	}

	if h.progressFactory != nil {
		reporter, rerr := h.progressFactory.NewReporter(callbackURL, requestID)
		if rerr != nil {
			h.log.Warn("invalid callbackUrl, disabling progress reporting", zap.Error(rerr))
		} else if reporter != nil {
			ctx = progress.ContextWithReporter(ctx, reporter)
		}
	}

	resp := h.handler.Handle(ctx, req)

	feedback, termErr := h.produceFeedback(resp, adapter, command)
	if termErr != nil {
		progress.Emit(ctx, progress.Event{
			Stage:     progress.StageFailed,
			Command:   string(command),
			Message:   termErr.userMessage,
			Error:     termErr.rawError,
			ErrorInfo: termErr.progressErrorInfo("Evaluation failed"),
		})

		if termErr.passthrough {
			for k, v := range termErr.header {
				for _, vv := range v {
					w.Header().Add(k, vv)
				}
			}
			w.Header().Set(muEdVersionHeader, version)
			w.WriteHeader(termErr.status)
			w.Write(termErr.body) //nolint:errcheck
			return
		}

		h.writeMuEdError(w, version, termErr.status, termErr.muEdCode, termErr.muEdTitle, termErr.muEdMessage, nil)
		return
	}

	// Carry the feedback itself on the completed event so that, when a
	// caller supplies callbackUrl, that callback genuinely fulfils the
	// µEd spec's "deliver feedback results to this URL" wording — even
	// though shimmy always takes the synchronous 200 path rather than
	// the spec's 202-Accepted deferred-delivery flow.
	progress.Emit(ctx, progress.Event{
		Stage:   progress.StageCompleted,
		Command: string(command),
		Message: "Feedback is ready.",
		Data:    map[string]any{"feedback": feedback},
	})

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set(muEdVersionHeader, version)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(feedback) //nolint:errcheck
}

// acceptsEventStream reports whether the caller opted in to an SSE
// streaming response via the Accept header.
func acceptsEventStream(r *http.Request) bool {
	return strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/event-stream")
}

// terminalError is the outcome of produceFeedback when feedback can't be
// produced. It carries everything the buffered and streaming paths each
// need to report the failure, so produceFeedback itself performs no
// writes and emits no events.
type terminalError struct {
	// passthrough replays the evaluation function's own non-2xx response
	// verbatim (buffered path only).
	passthrough bool
	header      http.Header
	status      int
	body        []byte

	// muEd* describe a shimmy-internal error. The buffered path passes
	// them straight to writeMuEdError; both paths also feed them into the
	// StageFailed event's ErrorInfo via progressErrorInfo.
	muEdCode    string
	muEdTitle   string
	muEdMessage string

	// userMessage and rawError are the human-facing line and raw detail
	// for the StageFailed progress event (Message / Error), and the
	// fallbacks progressErrorInfo uses for the ErrorInfo message / trace.
	userMessage string
	rawError    string
}

// progressErrorInfo maps the terminalError to the structured error object
// carried on the StageFailed progress event and emitted as the SSE
// "failed" frame's `error` (shaped like the spec's ErrorResponse). title
// falls back to fallbackTitle when the error has no µEd title (e.g. a
// worker-response passthrough), so the object always satisfies
// ErrorResponse, whose only required field is title.
func (e *terminalError) progressErrorInfo(fallbackTitle string) *progress.ErrorInfo {
	title := e.muEdTitle
	if title == "" {
		title = fallbackTitle
	}
	msg := e.muEdMessage
	if msg == "" {
		msg = e.userMessage
	}
	return &progress.ErrorInfo{
		Title:   title,
		Message: msg,
		Code:    e.muEdCode,
		Trace:   e.rawError,
	}
}

// produceFeedback turns a runtime response into muEd feedback via the
// resolved version adapter, or a terminalError describing why it couldn't.
// It is pure: no writes, no progress events.
func (h *MuEdHandler) produceFeedback(resp runtime.Response, adapter runtime.MuEdAdapter, command runtime.Command) ([]map[string]any, *terminalError) {
	if resp.StatusCode != http.StatusOK {
		return nil, &terminalError{
			passthrough: true,
			header:      resp.Header,
			status:      resp.StatusCode,
			body:        resp.Body,
			userMessage: muEdErrorMessageFromBody(resp.Body),
			rawError:    string(resp.Body),
		}
	}

	var respBody map[string]any
	if err := json.Unmarshal(resp.Body, &respBody); err != nil {
		return nil, &terminalError{
			status:      http.StatusInternalServerError,
			muEdCode:    "INTERNAL_ERROR",
			muEdTitle:   "Internal server error",
			muEdMessage: "failed to parse response",
			userMessage: "We couldn't evaluate your answer. Please try again.",
			rawError:    fmt.Sprintf("failed to parse response: %v", err),
		}
	}

	result, ok := respBody["result"].(map[string]any)
	if !ok {
		return nil, &terminalError{
			status:      http.StatusInternalServerError,
			muEdCode:    "INTERNAL_ERROR",
			muEdTitle:   "Internal server error",
			muEdMessage: "invalid response from evaluation function",
			userMessage: "We couldn't evaluate your answer. Please try again.",
			rawError:    "invalid response from evaluation function",
		}
	}

	feedback, err := adapter.EncodeEvaluateFeedback(command, result)
	if err != nil {
		return nil, &terminalError{
			status:      http.StatusInternalServerError,
			muEdCode:    "INTERNAL_ERROR",
			muEdTitle:   "Internal server error",
			muEdMessage: "failed to build feedback",
			userMessage: "We couldn't evaluate your answer. Please try again.",
			rawError:    fmt.Sprintf("failed to build feedback: %v", err),
		}
	}
	return feedback, nil
}

// serveEvaluateStream handles a POST /evaluate request that opted in to
// SSE streaming. The streaming scaffold (headers, reporter, heartbeats,
// terminal frame) lives in streamProgress; this only supplies the run
// step. Because the 200 is committed before the worker runs, every
// post-Handle outcome — including an internal error — becomes a "failed"
// frame, never an HTTP error.
func (h *MuEdHandler) serveEvaluateStream(
	ctx context.Context,
	w http.ResponseWriter,
	req runtime.Request,
	adapter runtime.MuEdAdapter,
	command runtime.Command,
	version string,
	callbackURL string,
	requestID string,
) {
	cmdLabel := "evaluate"
	if command == runtime.CommandPreview {
		cmdLabel = "preview"
	}

	h.streamProgress(ctx, w, cmdLabel, string(command), "Feedback is ready.", version, callbackURL, requestID,
		func(ctx context.Context) (map[string]any, *terminalError) {
			resp := h.handler.Handle(ctx, req)
			feedback, termErr := h.produceFeedback(resp, adapter, command)
			if termErr != nil {
				return nil, termErr
			}
			return map[string]any{"feedback": feedback}, nil
		})
}

// muEdErrorMessageFromBody best-effort extracts a human-readable message
// from a JSON error body of the shape {"error": {"message": "..."}}.
func muEdErrorMessageFromBody(body []byte) string {
	const fallback = "We couldn't evaluate your answer. Please try again."

	var errBody map[string]any
	if err := json.Unmarshal(body, &errBody); err != nil {
		return fallback
	}

	errObj, ok := errBody["error"].(map[string]any)
	if !ok {
		return fallback
	}

	msg, ok := errObj["message"].(string)
	if !ok || msg == "" {
		return fallback
	}

	return msg
}

// ServeHealth handles GET /evaluate/health.
func (h *MuEdHandler) ServeHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(muEdRequestIDHeader, resolveRequestID(r))

	if !h.checkAuth(w, r) {
		return
	}

	version, adapter, ok := h.checkMuEdVersion(w, r)
	if !ok {
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp, err := h.runtime.Handle(r.Context(), runtime.EvaluationRequest{
		Command: runtime.CommandEvaluateHealth,
		Data:    map[string]any{},
	})
	if err != nil {
		h.writeMuEdError(w, version, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", "health check failed", nil)
		return
	}

	legacyResult, ok := resp["result"].(map[string]any)
	if !ok {
		h.writeMuEdError(w, version, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", "invalid health response", nil)
		return
	}

	result := adapter.EncodeHealth(legacyResult, h.streamingEnabled())

	statusCode := http.StatusOK
	if s, ok := result["status"].(string); ok && s == "UNAVAILABLE" {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set(muEdVersionHeader, version)
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(result) //nolint:errcheck
}

func NewMuEdEvaluateRoute(handler *MuEdHandler) server.HttpHandlerResult {
	return server.AsHttpHandler("/evaluate", http.HandlerFunc(handler.ServeEvaluate))
}

func NewMuEdEvaluateHealthRoute(handler *MuEdHandler) server.HttpHandlerResult {
	return server.AsHttpHandler("/evaluate/health", http.HandlerFunc(handler.ServeHealth))
}
