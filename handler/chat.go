package handler

import (
	"net/http"

	"github.com/lambda-feedback/shimmy/internal/server"
)

// ServeChat handles POST /chat.
func (h *MuEdHandler) ServeChat(w http.ResponseWriter, r *http.Request) {
	if !h.checkAuth(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// ServeChatHealth handles GET /chat/health.
func (h *MuEdHandler) ServeChatHealth(w http.ResponseWriter, r *http.Request) {
	if !h.checkAuth(w, r) {
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func NewMuEdChatRoute(handler *MuEdHandler) server.HttpHandlerResult {
	return server.AsHttpHandler("/chat", http.HandlerFunc(handler.ServeChat))
}

func NewMuEdChatHealthRoute(handler *MuEdHandler) server.HttpHandlerResult {
	return server.AsHttpHandler("/chat/health", http.HandlerFunc(handler.ServeChatHealth))
}
