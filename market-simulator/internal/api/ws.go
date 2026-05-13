package api

import "net/http"

// LiveEvents will upgrade to WebSocket for sim ticks and fills (Redis-backed).
func (h *Handler) LiveEvents(w http.ResponseWriter, _ *http.Request) {
	notImplemented(w, "GET /api/v1/live/stream — WebSocket upgrade pending")
}
