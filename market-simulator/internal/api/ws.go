package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var liveUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// LiveEvents upgrades to WebSocket and fans out Redis sim.events to the dashboard.
func (h *Handler) LiveEvents(w http.ResponseWriter, r *http.Request) {
	if h.Redis == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "REDIS_URL required for live stream"})
		return
	}

	var filter uuid.UUID
	if sid := r.URL.Query().Get("simulation_id"); sid != "" {
		id, err := uuid.Parse(sid)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid simulation_id"})
			return
		}
		filter = id
	}

	conn, err := liveUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	pubsub := h.Redis.Subscribe(r.Context(), "sim.events")
	defer pubsub.Close()

	ch := pubsub.Channel()
	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var raw map[string]any
			if err := json.Unmarshal([]byte(msg.Payload), &raw); err != nil {
				continue
			}
			if filter != uuid.Nil {
				if s, _ := raw["simulation_id"].(string); s != filter.String() {
					continue
				}
			}
			out := map[string]any{
				"type": mapLiveType(raw),
				"data": raw,
			}
			_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteJSON(out); err != nil {
				return
			}
		}
	}
}

func mapLiveType(raw map[string]any) string {
	ev, _ := raw["event"].(string)
	switch ev {
	case "sim.tick", "sim.tick.processed", "sim.started", "sim.resumed", "sim.checkpoint":
		return "tick"
	case "order.filled", "order.rejected":
		return "order"
	case "sim.completed":
		return "completed"
	case "agent.disqualified":
		return "disqualified"
	default:
		if ev != "" {
			return ev
		}
		return "event"
	}
}
