package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/agentic-sim-trading/market-simulator/internal/market"
	"github.com/agentic-sim-trading/market-simulator/internal/portfolio"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Handler wires HTTP to subsystems (expanded through roadmap Steps 8–10).
type Handler struct {
	DB        *pgxpool.Pool
	Redis     *redis.Client
	Market    *market.Data
	Quotes    *market.QuoteProvider
	Portfolio *portfolio.Manager
}

func notImplemented(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_ = json.NewEncoder(w).Encode(map[string]string{"detail": msg})
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	body := map[string]any{"status": "ok", "service": "market-simulator"}
	if h.DB != nil {
		if err := h.DB.Ping(r.Context()); err != nil {
			body["postgres"] = "error"
		} else {
			body["postgres"] = "ok"
		}
	}
	if h.Redis != nil {
		if err := h.Redis.Ping(r.Context()).Err(); err != nil {
			body["redis"] = "error"
		} else {
			body["redis"] = "ok"
		}
	}
	_ = json.NewEncoder(w).Encode(body)
}

func (h *Handler) PlaceOrder(w http.ResponseWriter, _ *http.Request) {
	notImplemented(w, "order placement wired in roadmap Step 8 (matching engine)")
}

func (h *Handler) GetPortfolio(w http.ResponseWriter, r *http.Request) {
	agentStr := chi.URLParam(r, "agentId")
	agentID, err := uuid.Parse(agentStr)
	if err != nil {
		http.Error(w, `{"detail":"invalid agent id"}`, http.StatusBadRequest)
		return
	}
	simStr := r.URL.Query().Get("simulation_id")
	if simStr == "" {
		http.Error(w, `{"detail":"query simulation_id required"}`, http.StatusBadRequest)
		return
	}
	simulationID, err := uuid.Parse(simStr)
	if err != nil {
		http.Error(w, `{"detail":"invalid simulation_id"}`, http.StatusBadRequest)
		return
	}
	pm := h.Portfolio
	if pm == nil {
		pm = portfolio.NewManager(h.DB)
	}
	row, err := pm.GetPortfolio(r.Context(), simulationID, agentID)
	if err != nil {
		http.Error(w, `{"detail":"portfolio not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(row)
}

func (h *Handler) GetQuote(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	qp := h.Quotes
	if qp == nil {
		qp = market.NewQuoteProvider(market.NewData(h.DB))
	}
	q, err := qp.Current(r.Context(), symbol)
	if err != nil {
		if err == market.ErrNoDatabase {
			notImplemented(w, "DATABASE_URL not set — cannot load OHLCV")
			return
		}
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, `{"detail":"no bars for symbol"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"detail":"quote failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(q)
}

func (h *Handler) GetOHLCV(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	days := 20
	if d := r.URL.Query().Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 && v <= 500 {
			days = v
		}
	}
	data := h.Market
	if data == nil {
		data = market.NewData(h.DB)
	}
	bars, err := data.BarsForSymbol(r.Context(), symbol, time.Now().UTC(), days)
	if err != nil {
		if err == market.ErrNoDatabase {
			notImplemented(w, "DATABASE_URL not set — cannot load OHLCV")
			return
		}
		http.Error(w, `{"detail":"query failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(bars)
}

func (h *Handler) ListOrders(w http.ResponseWriter, _ *http.Request) {
	notImplemented(w, "GET /api/v1/orders/{agentId} — persistence-backed history (Step 8)")
}

func (h *Handler) CancelOrder(w http.ResponseWriter, _ *http.Request) {
	notImplemented(w, "DELETE /api/v1/orders/{orderId} — cancel pending (Step 8)")
}

func (h *Handler) CreateSimulation(w http.ResponseWriter, _ *http.Request) {
	notImplemented(w, "POST /api/v1/simulations — create simulation (Step 9)")
}

func (h *Handler) StartSimulation(w http.ResponseWriter, _ *http.Request) {
	notImplemented(w, "POST /api/v1/simulations/{id}/start — clock start (Step 9)")
}

func (h *Handler) PauseSimulation(w http.ResponseWriter, _ *http.Request) {
	notImplemented(w, "POST /api/v1/simulations/{id}/pause — clock pause (Step 9)")
}

func (h *Handler) PutSimSpeed(w http.ResponseWriter, _ *http.Request) {
	notImplemented(w, "PUT /api/v1/sim/speed — scheduling multiplier (future)")
}

func (h *Handler) GetLeaderboard(w http.ResponseWriter, _ *http.Request) {
	notImplemented(w, "GET /api/v1/leaderboard/{simId} — rankings (Step 9)")
}
