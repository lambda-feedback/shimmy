package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.uber.org/zap"

	"github.com/lambda-feedback/shimmy/internal/progress"
	"github.com/lambda-feedback/shimmy/internal/server"
	"github.com/lambda-feedback/shimmy/runtime"
)

// ServeChat handles POST /chat.
func (h *MuEdHandler) ServeChat(w http.ResponseWriter, r *http.Request) {
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

	reqData, err := adapter.DecodeChat(body)
	if err != nil {
		h.writeMuEdError(w, version, http.StatusBadRequest, "VALIDATION_ERROR", "Bad request", err.Error(), nil)
		return
	}

	// The adapter's DecodeChat deliberately does not surface transport
	// concerns like callbackUrl, so pull it from the raw body here. A parse
	// failure is impossible in practice — DecodeChat already parsed the
	// same bytes — so a miss just means no out-of-band progress delivery.
	var callbackURL string
	var chatReq runtime.MuEdChatRequest
	if json.Unmarshal(body, &chatReq) == nil && chatReq.CallbackUrl != nil {
		callbackURL = *chatReq.CallbackUrl
	}

	streaming := h.streamingEnabled() && acceptsEventStream(r) && adapter.SupportsStreaming()
	if streaming {
		if _, ok := w.(http.Flusher); !ok {
			h.log.Warn("response writer is not a flusher; serving buffered response")
			streaming = false
		}
	}

	ctx := r.Context()

	if streaming {
		h.serveChatStream(ctx, w, reqData, adapter, version, callbackURL, requestID)
		return
	}

	if callbackURL != "" && h.progressFactory != nil {
		reporter, rerr := h.progressFactory.NewReporter(callbackURL, requestID)
		if rerr != nil {
			h.log.Warn("invalid callbackUrl, disabling progress reporting", zap.Error(rerr))
		} else if reporter != nil {
			ctx = progress.ContextWithReporter(ctx, reporter)
		}
	}

	resp, err := h.runtime.Chat(ctx, runtime.ChatRequest{Data: reqData})
	chatResp, termErr := h.produceChatOutput(resp, err, adapter)
	if termErr != nil {
		progress.Emit(ctx, progress.Event{
			Stage:     progress.StageFailed,
			Command:   string(runtime.CommandChat),
			Message:   termErr.userMessage,
			Error:     termErr.rawError,
			ErrorInfo: termErr.progressErrorInfo("Chat failed"),
		})
		h.writeMuEdError(w, version, termErr.status, termErr.muEdCode, termErr.muEdTitle, termErr.muEdMessage, nil)
		return
	}

	progress.Emit(ctx, progress.Event{
		Stage:   progress.StageCompleted,
		Command: string(runtime.CommandChat),
		Message: "Response is ready.",
		Data:    chatResp,
	})

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set(muEdVersionHeader, version)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(chatResp) //nolint:errcheck
}

// serveChatStream handles a POST /chat request that opted in to SSE
// streaming. The streaming scaffold lives in streamProgress; this only
// supplies the run step.
func (h *MuEdHandler) serveChatStream(
	ctx context.Context,
	w http.ResponseWriter,
	reqData map[string]any,
	adapter runtime.MuEdAdapter,
	version string,
	callbackURL string,
	requestID string,
) {
	h.streamProgress(ctx, w, "chat", string(runtime.CommandChat), "Response is ready.", version, callbackURL, requestID,
		func(ctx context.Context) (map[string]any, *terminalError) {
			resp, err := h.runtime.Chat(ctx, runtime.ChatRequest{Data: reqData})
			chatResp, termErr := h.produceChatOutput(resp, err, adapter)
			if termErr != nil {
				return nil, termErr
			}
			return chatResp, nil
		})
}

// produceChatOutput turns a runtime chat response into the µEd chat
// response object via the resolved version adapter, or a terminalError
// describing why it couldn't. It is pure: no writes, no progress events.
// Unlike produceFeedback there is no worker-non-200 passthrough —
// runtime.Chat returns (response, error), not an HTTP status — so every
// failure is a 500-class terminalError.
func (h *MuEdHandler) produceChatOutput(resp runtime.ChatResponse, chatErr error, adapter runtime.MuEdAdapter) (map[string]any, *terminalError) {
	newErr := func(muEdMessage, rawError string) *terminalError {
		return &terminalError{
			status:      http.StatusInternalServerError,
			muEdCode:    "INTERNAL_ERROR",
			muEdTitle:   "Internal server error",
			muEdMessage: muEdMessage,
			userMessage: "We couldn't generate a response. Please try again.",
			rawError:    rawError,
		}
	}

	if chatErr != nil {
		return nil, newErr("chat failed", chatErr.Error())
	}

	resultMap, ok := resp.Data["result"].(map[string]any)
	if !ok {
		return nil, newErr("invalid response from chat function", "invalid response from chat function")
	}

	chatResp, err := adapter.EncodeChat(resultMap)
	if err != nil {
		return nil, newErr(err.Error(), fmt.Sprintf("invalid chat response: %v", err))
	}
	return chatResp, nil
}

// ServeChatHealth handles GET /chat/health.
func (h *MuEdHandler) ServeChatHealth(w http.ResponseWriter, r *http.Request) {
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

	resp, err := h.runtime.ChatHealth(r.Context())
	if err != nil {
		h.writeMuEdError(w, version, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", "chat health check failed", nil)
		return
	}

	resultMap, ok := resp.Data["result"].(map[string]any)
	if !ok {
		h.writeMuEdError(w, version, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", "invalid chat health response", nil)
		return
	}

	healthResp := adapter.EncodeChatHealth(resultMap, h.streamingEnabled())

	statusCode := http.StatusOK
	if status, ok := healthResp["status"].(string); ok && status == string(runtime.MuEdChatHealthStatusUnavailable) {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set(muEdVersionHeader, version)
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(healthResp) //nolint:errcheck
}

func NewMuEdChatRoute(handler *MuEdHandler) server.HttpHandlerResult {
	return server.AsHttpHandler("/chat", http.HandlerFunc(handler.ServeChat))
}

func NewMuEdChatHealthRoute(handler *MuEdHandler) server.HttpHandlerResult {
	return server.AsHttpHandler("/chat/health", http.HandlerFunc(handler.ServeChatHealth))
}
