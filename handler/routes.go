package handler

import (
	"net/http"

	"github.com/lambda-feedback/shimmy/internal/server"
)

func NewLegacyRoute(handler *CommandHandler) server.HttpHandlerResult {
	return server.AsHttpHandler("/", handler)
}

func NewCommandRoute(handler *CommandHandler) server.HttpHandlerResult {
	return server.AsHttpHandler("/{command}", handler)
}

func NewHealthRoute() server.HttpHandlerResult {
	return server.AsHttpHandler("/health", http.HandlerFunc(HealthHandler))
}

func NewMuEdEvaluateRoute(handler *MuEdHandler) server.HttpHandlerResult {
	return server.AsHttpHandler("POST /evaluate", http.HandlerFunc(handler.ServeEvaluate))
}

func NewMuEdHealthRoute(handler *MuEdHandler) server.HttpHandlerResult {
	return server.AsHttpHandler("GET /evaluate/health", http.HandlerFunc(handler.ServeHealth))
}
