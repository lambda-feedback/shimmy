package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/lambda-feedback/shimmy/internal/server"
	"github.com/lambda-feedback/shimmy/runtime"
)

// ServeChat handles POST /chat.
func (h *MuEdHandler) ServeChat(w http.ResponseWriter, r *http.Request) {
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

	resp, err := h.runtime.Chat(r.Context(), runtime.ChatRequest{Data: reqData})
	if err != nil {
		h.writeMuEdError(w, version, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", "chat failed", nil)
		return
	}

	resultMap, ok := resp.Data["result"].(map[string]any)
	if !ok {
		h.writeMuEdError(w, version, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", "invalid response from chat function", nil)
		return
	}

	chatResp, err := runtime.MuEdToChatResponse(resultMap)
	if err != nil {
		h.writeMuEdError(w, version, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", err.Error(), nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set(muEdVersionHeader, version)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(chatResp) //nolint:errcheck
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
	if healthResp.Status == runtime.MuEdChatHealthStatusUnavailable {
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
