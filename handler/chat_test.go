package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- ServeChat tests ---

func TestServeChat_NotImplemented(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/chat", nil)
	w := httptest.NewRecorder()
	newMuEdHandler(nil, nil, "").ServeChat(w, req)
	assert.Equal(t, http.StatusNotImplemented, w.Result().StatusCode)
}

func TestServeChat_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/chat", nil)
	req.Header.Set("api-key", "wrong")
	w := httptest.NewRecorder()
	newMuEdHandler(nil, nil, "secret").ServeChat(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
}

func TestServeChat_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/chat", nil)
	w := httptest.NewRecorder()
	newMuEdHandler(nil, nil, "").ServeChat(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Result().StatusCode)
}

// --- ServeChatHealth tests ---

func TestServeChatHealth_NotImplemented(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/chat/health", nil)
	w := httptest.NewRecorder()
	newMuEdHandler(nil, nil, "").ServeChatHealth(w, req)
	assert.Equal(t, http.StatusNotImplemented, w.Result().StatusCode)
}

func TestServeChatHealth_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/chat/health", nil)
	req.Header.Set("api-key", "wrong")
	w := httptest.NewRecorder()
	newMuEdHandler(nil, nil, "secret").ServeChatHealth(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
}

func TestServeChatHealth_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/chat/health", nil)
	w := httptest.NewRecorder()
	newMuEdHandler(nil, nil, "").ServeChatHealth(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Result().StatusCode)
}
