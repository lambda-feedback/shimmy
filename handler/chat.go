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
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	var chatReq runtime.MuEdChatRequest
	if err := json.Unmarshal(body, &chatReq); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	reqData, err := runtime.MuEdBuildChatRequest(chatReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := h.runtime.Handle(r.Context(), runtime.EvaluationRequest{
		Command: runtime.CommandChat,
		Data:    reqData,
	})
	if err != nil {
		http.Error(w, "chat failed", http.StatusInternalServerError)
		return
	}

	resultMap, ok := resp["result"].(map[string]any)
	if !ok {
		http.Error(w, "invalid response from chat function", http.StatusInternalServerError)
		return
	}

	chatResp, err := runtime.MuEdToChatResponse(resultMap)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	resp, err := h.runtime.Handle(r.Context(), runtime.EvaluationRequest{
		Command: runtime.CommandChatHealth,
		Data:    map[string]any{},
	})
	if err != nil {
		http.Error(w, "chat health check failed", http.StatusInternalServerError)
		return
	}

	resultMap, ok := resp["result"].(map[string]any)
	if !ok {
		http.Error(w, "invalid chat health response", http.StatusInternalServerError)
		return
	}

	healthResp := runtime.MuEdToChatHealthResponse(resultMap)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set(muEdVersionHeader, version)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(healthResp) //nolint:errcheck
}

func NewMuEdChatRoute(handler *MuEdHandler) server.HttpHandlerResult {
	return server.AsHttpHandler("/chat", http.HandlerFunc(handler.ServeChat))
}

func NewMuEdChatHealthRoute(handler *MuEdHandler) server.HttpHandlerResult {
	return server.AsHttpHandler("/chat/health", http.HandlerFunc(handler.ServeChatHealth))
}
