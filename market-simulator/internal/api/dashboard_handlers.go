package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/agentic-sim-trading/market-simulator/internal/dashboard"
	"github.com/agentic-sim-trading/market-simulator/internal/orders"
	"github.com/agentic-sim-trading/market-simulator/internal/portfolio"
	"github.com/agentic-sim-trading/market-simulator/internal/simulation"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *Handler) ListSimulations(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "DATABASE_URL required"})
		return
	}
	limit := 50
	if ls := r.URL.Query().Get("limit"); ls != "" {
		if v, err := strconv.Atoi(ls); err == nil && v > 0 {
			limit = v
		}
	}
	items, err := simulation.List(r.Context(), h.DB, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "list failed"})
		return
	}
	if items == nil {
		items = []simulation.Summary{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"simulations": items})
}

func (h *Handler) DashboardEquityCurves(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "DATABASE_URL required"})
		return
	}
	simID, err := uuid.Parse(chi.URLParam(r, "simId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid simulation id"})
		return
	}
	curves, err := dashboard.EquityCurves(r.Context(), h.DB, simID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "equity curves failed"})
		return
	}
	if curves == nil {
		curves = []dashboard.AgentEquityCurve{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"simulation_id": simID.String(),
		"curves":        curves,
	})
}

func (h *Handler) DashboardRecentOrders(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "DATABASE_URL required"})
		return
	}
	simID, err := uuid.Parse(chi.URLParam(r, "simId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid simulation id"})
		return
	}
	limit := 20
	if ls := r.URL.Query().Get("limit"); ls != "" {
		if v, err := strconv.Atoi(ls); err == nil && v > 0 {
			limit = v
		}
	}
	orders, err := dashboard.RecentOrders(r.Context(), h.DB, simID, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "orders query failed"})
		return
	}
	if orders == nil {
		orders = []dashboard.OrderRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"simulation_id": simID.String(),
		"orders":        orders,
	})
}

func (h *Handler) DashboardListAgents(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "DATABASE_URL required"})
		return
	}
	simID, err := uuid.Parse(chi.URLParam(r, "simId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid simulation id"})
		return
	}
	agents, err := dashboard.ListAgents(r.Context(), h.DB, simID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "agents query failed"})
		return
	}
	if agents == nil {
		agents = []dashboard.AgentSummary{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"simulation_id": simID.String(),
		"agents":        agents,
	})
}

func (h *Handler) DashboardAgentPortfolio(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "DATABASE_URL required"})
		return
	}
	if h.Market == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "market data not configured"})
		return
	}
	simID, err := uuid.Parse(chi.URLParam(r, "simId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid simulation id"})
		return
	}
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid agent id"})
		return
	}

	asOf, err := h.resolvePortfolioMarkDate(r.Context(), simID)
	if err != nil {
		switch {
		case errors.Is(err, simulation.ErrNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"detail": err.Error()})
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		}
		return
	}

	pm := h.Portfolio
	if pm == nil {
		pm = portfolio.NewManager(h.DB)
	}
	detail, err := pm.GetPortfolioDetail(r.Context(), simID, agentID, asOf, h.Market)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "portfolio not found"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) DashboardAgentOrders(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "DATABASE_URL required"})
		return
	}
	simID, err := uuid.Parse(chi.URLParam(r, "simId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid simulation id"})
		return
	}
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid agent id"})
		return
	}
	limit := 100
	if ls := r.URL.Query().Get("limit"); ls != "" {
		if v, err := strconv.Atoi(ls); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}
	items, err := orders.ListByAgent(r.Context(), h.DB, simID, agentID, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "query failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) DashboardAgentMetrics(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "DATABASE_URL required"})
		return
	}
	simID, err := uuid.Parse(chi.URLParam(r, "simId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid simulation id"})
		return
	}
	agentID, err := uuid.Parse(chi.URLParam(r, "agentId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid agent id"})
		return
	}
	metrics, err := dashboard.BuildAgentMetrics(r.Context(), h.DB, simID, agentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "metrics failed"})
		return
	}
	writeJSON(w, http.StatusOK, metrics)
}
