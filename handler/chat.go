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

	version, ok := h.checkMuEdVersion(w, r)
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

	var chatReq runtime.MuEdChatRequest
	if err := json.Unmarshal(body, &chatReq); err != nil {
		h.writeMuEdError(w, version, http.StatusBadRequest, "VALIDATION_ERROR", "Bad request", "invalid request body", nil)
		return
	}

	reqData, err := runtime.MuEdBuildChatRequest(chatReq)
	if err != nil {
		h.writeMuEdError(w, version, http.StatusBadRequest, "VALIDATION_ERROR", "Bad request", err.Error(), nil)
		return
	}

	var callbackURL string
	if chatReq.CallbackUrl != nil {
		callbackURL = *chatReq.CallbackUrl
	}

	streaming := h.streamingCapable && h.config.Progress.Stream.Enabled && acceptsEventStream(r)
	if streaming {
		if _, ok := w.(http.Flusher); !ok {
			h.log.Warn("response writer is not a flusher; serving buffered response")
			streaming = false
		}
	}

	ctx := r.Context()

	if streaming {
		h.serveChatStream(ctx, w, reqData, version, callbackURL, requestID)
		return
	}

	if callbackURL != "" {
		reporter, rerr := h.progressFactory.NewReporter(callbackURL, requestID)
		if rerr != nil {
			h.log.Warn("invalid callbackUrl, disabling progress reporting", zap.Error(rerr))
		} else if reporter != nil {
			ctx = progress.ContextWithReporter(ctx, reporter)
		}
	}

	resp, err := h.runtime.Chat(ctx, runtime.ChatRequest{Data: reqData})
	output, metadata, termErr := h.produceChatOutput(resp, err)
	if termErr != nil {
		progress.Emit(ctx, progress.Event{
			Stage:   progress.StageFailed,
			Command: string(runtime.CommandChat),
			Message: termErr.userMessage,
			Error:   termErr.rawError,
		})
		h.writeMuEdError(w, version, termErr.status, termErr.muEdCode, termErr.muEdTitle, termErr.muEdMessage, nil)
		return
	}

	chatResp := map[string]any{"output": output}
	if metadata != nil {
		chatResp["metadata"] = metadata
	}

	progress.Emit(ctx, progress.Event{
		Stage:   progress.StageCompleted,
		Command: string(runtime.CommandChat),
		Message: "Response is ready.",
		Data:    map[string]any{"output": output, "metadata": metadata},
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
	version string,
	callbackURL string,
	requestID string,
) {
	h.streamProgress(ctx, w, "chat", string(runtime.CommandChat), "Response is ready.", version, callbackURL, requestID,
		func(ctx context.Context) (map[string]any, *terminalError) {
			resp, err := h.runtime.Chat(ctx, runtime.ChatRequest{Data: reqData})
			output, metadata, termErr := h.produceChatOutput(resp, err)
			if termErr != nil {
				return nil, termErr
			}
			data := map[string]any{"output": output}
			if metadata != nil {
				data["metadata"] = metadata
			}
			return data, nil
		})
}

// produceChatOutput turns a runtime chat response into the µEd output
// object (+ optional metadata), or a terminalError describing why it
// couldn't. It is pure: no writes, no progress events. Unlike
// produceFeedback there is no worker-non-200 passthrough — runtime.Chat
// returns (response, error), not an HTTP status — so every failure is a
// 500-class terminalError.
func (h *MuEdHandler) produceChatOutput(resp runtime.ChatResponse, chatErr error) (output, metadata map[string]any, _ *terminalError) {
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
		return nil, nil, newErr("chat failed", chatErr.Error())
	}

	resultMap, ok := resp.Data["result"].(map[string]any)
	if !ok {
		return nil, nil, newErr("invalid response from chat function", "invalid response from chat function")
	}

	chatResp, err := runtime.MuEdToChatResponse(resultMap)
	if err != nil {
		return nil, nil, newErr(err.Error(), fmt.Sprintf("invalid chat response: %v", err))
	}

	output, _ = chatResp["output"].(map[string]any)
	metadata, _ = chatResp["metadata"].(map[string]any)
	return output, metadata, nil
}

// ServeChatHealth handles GET /chat/health.
func (h *MuEdHandler) ServeChatHealth(w http.ResponseWriter, r *http.Request) {
	if !h.checkAuth(w, r) {
		return
	}

	version, ok := h.checkMuEdVersion(w, r)
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

	healthResp := runtime.MuEdToChatHealthResponse(resultMap)

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
