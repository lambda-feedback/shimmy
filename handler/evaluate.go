package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/lambda-feedback/shimmy/config"
	"github.com/lambda-feedback/shimmy/internal/server"
	"github.com/lambda-feedback/shimmy/runtime"
)

const muEdVersionHeader = "X-Api-Version"

type MuEdHandlerParams struct {
	fx.In

	Handler  runtime.Handler
	Runtime  runtime.Runtime
	Registry *runtime.MuEdRegistry
	Config   config.Config
	Log      *zap.Logger
}

type MuEdHandler struct {
	handler  runtime.Handler
	runtime  runtime.Runtime
	registry *runtime.MuEdRegistry
	config   config.Config
	log      *zap.Logger
}

func NewMuEdHandler(params MuEdHandlerParams) *MuEdHandler {
	return &MuEdHandler{
		handler:  params.Handler,
		runtime:  params.Runtime,
		registry: params.Registry,
		config:   params.Config,
		log:      params.Log,
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

	resp := h.handler.Handle(r.Context(), req)

	if resp.StatusCode != http.StatusOK {
		for k, v := range resp.Header {
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}
		w.Header().Set(muEdVersionHeader, version)
		w.WriteHeader(resp.StatusCode)
		w.Write(resp.Body) //nolint:errcheck
		return
	}

	var respBody map[string]any
	if err := json.Unmarshal(resp.Body, &respBody); err != nil {
		h.writeMuEdError(w, version, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", "failed to parse response", nil)
		return
	}

	result, ok := respBody["result"].(map[string]any)
	if !ok {
		h.writeMuEdError(w, version, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", "invalid response from evaluation function", nil)
		return
	}

	feedback, err := adapter.EncodeEvaluateFeedback(command, result)
	if err != nil {
		h.writeMuEdError(w, version, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error", "failed to build feedback", nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set(muEdVersionHeader, version)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(feedback) //nolint:errcheck
}

// ServeHealth handles GET /evaluate/health.
func (h *MuEdHandler) ServeHealth(w http.ResponseWriter, r *http.Request) {
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

	result := adapter.EncodeHealth(legacyResult)

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
