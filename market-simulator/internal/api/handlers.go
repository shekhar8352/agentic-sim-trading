package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/agentic-sim-trading/market-simulator/internal/agents"
	"github.com/agentic-sim-trading/market-simulator/internal/clock"
	"github.com/agentic-sim-trading/market-simulator/internal/leaderboard"
	"github.com/agentic-sim-trading/market-simulator/internal/market"
	"github.com/agentic-sim-trading/market-simulator/internal/orders"
	"github.com/agentic-sim-trading/market-simulator/internal/portfolio"
	"github.com/agentic-sim-trading/market-simulator/internal/simulation"
	"github.com/agentic-sim-trading/market-simulator/internal/validation"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var errSimulationNoAsOf = errors.New("simulation has no as_of_date yet; POST .../start first")

// Handler wires HTTP to subsystems (expanded through roadmap Steps 8–10).
type Handler struct {
	DB        *pgxpool.Pool
	Redis     *redis.Client
	Market    *market.Data
	Portfolio *portfolio.Manager
	Clocks    *clock.Registry
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func notImplemented(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{"detail": msg})
}

func writeSimErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, simulation.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": err.Error()})
	case errors.Is(err, clock.ErrClockCompleted):
		writeJSON(w, http.StatusConflict, map[string]string{"detail": err.Error()})
	case errors.Is(err, clock.ErrNoTradingDays):
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
	case errors.Is(err, clock.ErrNoActiveClock):
		writeJSON(w, http.StatusConflict, map[string]string{"detail": err.Error()})
	case errors.Is(err, clock.ErrClockNotRunning):
		writeJSON(w, http.StatusConflict, map[string]string{"detail": err.Error()})
	case errors.Is(err, clock.ErrDatabaseRequired):
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "internal error"})
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, body)
}

func (h *Handler) PlaceOrder(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "DATABASE_URL required"})
		return
	}
	if h.Market == nil || h.Clocks == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "market data or clock registry not configured"})
		return
	}

	var req struct {
		SimulationID uuid.UUID `json:"simulation_id"`
		AgentID      uuid.UUID `json:"agent_id"`
		Symbol       string    `json:"symbol"`
		OrderType    string    `json:"order_type"`
		Side         string    `json:"side"`
		Quantity     int       `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid JSON body"})
		return
	}
	req.Symbol = strings.TrimSpace(req.Symbol)
	req.OrderType = strings.ToLower(strings.TrimSpace(req.OrderType))
	req.Side = strings.ToLower(strings.TrimSpace(req.Side))

	if req.SimulationID == uuid.Nil || req.AgentID == uuid.Nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "simulation_id and agent_id required"})
		return
	}
	if agentAccessDeniedFromRequest(w, r, req.AgentID) {
		return
	}
	if err := validation.MarketOrder(req.Symbol, req.OrderType, req.Side, req.Quantity); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}

	c, ok := h.Clocks.ActiveClock(req.SimulationID)
	if !ok || c.Status != "running" {
		writeJSON(w, http.StatusConflict, map[string]string{"detail": "simulation clock must be running"})
		return
	}

	if !h.Clocks.TryAcceptOrderSubmission(req.SimulationID, req.AgentID) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"detail": "order submission limit (10) reached until next processed tick"})
		return
	}

	asOf := c.CurrentDate.UTC().Truncate(24 * time.Hour)
	matchOn, err := h.Market.NextTradingDay(r.Context(), asOf)
	if err != nil || matchOn.IsZero() {
		h.Clocks.RollbackOrderSubmission(req.SimulationID, req.AgentID)
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "no next trading day after current simulation date"})
		return
	}

	id, err := orders.InsertPending(r.Context(), h.DB, req.SimulationID, req.AgentID, req.Symbol, req.OrderType, req.Side, req.Quantity, matchOn)
	if err != nil {
		h.Clocks.RollbackOrderSubmission(req.SimulationID, req.AgentID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "failed to persist order"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":            id.String(),
		"match_on_date": matchOn.UTC().Format(time.DateOnly),
		"status":        orders.StatusPending,
	})
}

func (h *Handler) GetPortfolio(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "DATABASE_URL required"})
		return
	}
	if h.Market == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "market data not configured"})
		return
	}
	agentStr := chi.URLParam(r, "agentId")
	agentID, err := uuid.Parse(agentStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid agent id"})
		return
	}
	if agentAccessDeniedFromRequest(w, r, agentID) {
		return
	}
	simStr := r.URL.Query().Get("simulation_id")
	if simStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "query simulation_id required"})
		return
	}
	simulationID, err := uuid.Parse(simStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid simulation_id"})
		return
	}

	asOf, err := h.resolvePortfolioMarkDate(r.Context(), simulationID)
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
	detail, err := pm.GetPortfolioDetail(r.Context(), simulationID, agentID, asOf, h.Market)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "portfolio not found"})
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) resolveSimulationAsOf(ctx context.Context, simID uuid.UUID) (time.Time, error) {
	if h.Clocks != nil {
		if t, ok := h.Clocks.AsOfDate(simID); ok {
			return t, nil
		}
	}
	if h.DB == nil {
		return time.Time{}, errors.New("database not configured")
	}
	row, err := simulation.Get(ctx, h.DB, simID)
	if err != nil {
		return time.Time{}, err
	}
	if row.AsOfDate != nil {
		return *row.AsOfDate, nil
	}
	return time.Time{}, errSimulationNoAsOf
}

// resolvePortfolioMarkDate chooses the calendar date for MTM (clock if loaded, else DB as_of, else simulation start).
func (h *Handler) resolvePortfolioMarkDate(ctx context.Context, simID uuid.UUID) (time.Time, error) {
	t, err := h.resolveSimulationAsOf(ctx, simID)
	if err == nil {
		return t.UTC().Truncate(24 * time.Hour), nil
	}
	if !errors.Is(err, errSimulationNoAsOf) {
		return time.Time{}, err
	}
	if h.DB == nil {
		return time.Time{}, errors.New("database not configured")
	}
	row, err := simulation.Get(ctx, h.DB, simID)
	if err != nil {
		return time.Time{}, err
	}
	return row.StartDate.UTC().Truncate(24 * time.Hour), nil
}

func (h *Handler) GetQuote(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	if err := validation.ListedSymbol(symbol); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	data := market.NewData(h.DB)
	qp := market.NewQuoteProvider(data)
	if sid := r.URL.Query().Get("simulation_id"); sid != "" {
		simID, err := uuid.Parse(sid)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid simulation_id"})
			return
		}
		asOf, err := h.resolveSimulationAsOf(r.Context(), simID)
		if err != nil {
			switch {
			case errors.Is(err, simulation.ErrNotFound):
				writeJSON(w, http.StatusNotFound, map[string]string{"detail": err.Error()})
			case errors.Is(err, errSimulationNoAsOf):
				writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
			default:
				writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
			}
			return
		}
		qp.AsOf = asOf
	}
	q, err := qp.Current(r.Context(), symbol)
	if err != nil {
		if err == market.ErrNoDatabase {
			notImplemented(w, "DATABASE_URL not set — cannot load OHLCV")
			return
		}
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"detail": "no bars for symbol"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "quote failed"})
		return
	}
	writeJSON(w, http.StatusOK, q)
}

func (h *Handler) GetOHLCV(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	if err := validation.ListedSymbol(symbol); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
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
	asOf := time.Now().UTC()
	if sid := r.URL.Query().Get("simulation_id"); sid != "" {
		simID, err := uuid.Parse(sid)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid simulation_id"})
			return
		}
		var err2 error
		asOf, err2 = h.resolveSimulationAsOf(r.Context(), simID)
		if err2 != nil {
			switch {
			case errors.Is(err2, simulation.ErrNotFound):
				writeJSON(w, http.StatusNotFound, map[string]string{"detail": err2.Error()})
			case errors.Is(err2, errSimulationNoAsOf):
				writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err2.Error()})
			default:
				writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err2.Error()})
			}
			return
		}
	}
	bars, err := data.BarsForSymbol(r.Context(), symbol, asOf, days)
	if err != nil {
		if err == market.ErrNoDatabase {
			notImplemented(w, "DATABASE_URL not set — cannot load OHLCV")
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "query failed"})
		return
	}
	writeJSON(w, http.StatusOK, bars)
}

func (h *Handler) ListOrders(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "DATABASE_URL required"})
		return
	}
	agentStr := chi.URLParam(r, "agentId")
	agentID, err := uuid.Parse(agentStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid agent id"})
		return
	}
	if agentAccessDeniedFromRequest(w, r, agentID) {
		return
	}
	simStr := r.URL.Query().Get("simulation_id")
	if simStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "query simulation_id required"})
		return
	}
	simulationID, err := uuid.Parse(simStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid simulation_id"})
		return
	}
	limit := 100
	if ls := r.URL.Query().Get("limit"); ls != "" {
		if v, err := strconv.Atoi(ls); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}
	items, err := orders.ListByAgent(r.Context(), h.DB, simulationID, agentID, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "query failed"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "DATABASE_URL required"})
		return
	}
	orderStr := chi.URLParam(r, "orderId")
	orderID, err := uuid.Parse(orderStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid order id"})
		return
	}
	simStr := r.URL.Query().Get("simulation_id")
	if simStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "query simulation_id required"})
		return
	}
	simulationID, err := uuid.Parse(simStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid simulation_id"})
		return
	}
	ownerID, pending, err := orders.PendingOrderAgentID(r.Context(), h.DB, simulationID, orderID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "lookup failed"})
		return
	}
	if !pending {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "pending order not found"})
		return
	}
	if agentAccessDeniedFromRequest(w, r, ownerID) {
		return
	}
	ok, err := orders.CancelPending(r.Context(), h.DB, simulationID, orderID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "cancel failed"})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "pending order not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"detail": "canceled"})
}

func (h *Handler) RegisterAgent(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "DATABASE_URL required"})
		return
	}
	simID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid simulation id"})
		return
	}
	if _, err := simulation.Get(r.Context(), h.DB, simID); err != nil {
		if errors.Is(err, simulation.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"detail": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "query failed"})
		return
	}

	var req struct {
		Name  string `json:"name"`
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid JSON body"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = "agent"
	}

	ctx := r.Context()
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "transaction failed"})
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	plainKey, err := agents.GenerateAPIKey()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "api key generation failed"})
		return
	}
	hash, err := agents.HashAPIKey(plainKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "api key hash failed"})
		return
	}

	var agentID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO agents (name, model, api_key_hash) VALUES ($1, $2, $3) RETURNING id
	`, req.Name, req.Model, hash).Scan(&agentID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "agent insert failed"})
		return
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO portfolios (simulation_id, agent_id, cash) VALUES ($1, $2, $3)
	`, simID, agentID, 1_000_000); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "portfolio insert failed"})
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "commit failed"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"agent_id":      agentID.String(),
		"simulation_id": simID.String(),
		"api_key":       plainKey,
	})
}

func (h *Handler) CreateSimulation(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "DATABASE_URL required"})
		return
	}
	var req struct {
		Name      string          `json:"name"`
		StartDate string          `json:"start_date"`
		EndDate   string          `json:"end_date"`
		Config    json.RawMessage `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid JSON body"})
		return
	}
	if req.Name == "" || req.StartDate == "" || req.EndDate == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "name, start_date, end_date required"})
		return
	}
	start, err := time.Parse(time.DateOnly, req.StartDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid start_date (use YYYY-MM-DD)"})
		return
	}
	end, err := time.Parse(time.DateOnly, req.EndDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid end_date (use YYYY-MM-DD)"})
		return
	}
	if end.Before(start) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "end_date must be on or after start_date"})
		return
	}
	id, err := simulation.Create(r.Context(), h.DB, req.Name, start, end, req.Config)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "failed to create simulation"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id.String(), "status": "paused"})
}

func (h *Handler) GetSimulation(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "DATABASE_URL required"})
		return
	}
	simID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid simulation id"})
		return
	}
	row, err := simulation.Get(r.Context(), h.DB, simID)
	if err != nil {
		if errors.Is(err, simulation.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"detail": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "query failed"})
		return
	}
	out := map[string]any{
		"id":                    row.ID.String(),
		"name":                  row.Name,
		"start_date":            row.StartDate.Format(time.DateOnly),
		"end_date":              row.EndDate.Format(time.DateOnly),
		"db_status":             row.Status,
		"config":                row.Config,
		"tick_speed_multiplier": simulation.TickSpeedMultiplier(row.Config),
		"has_as_of":             row.AsOfDate != nil,
		"clock_loaded":          false,
	}
	if row.AsOfDate != nil {
		out["as_of_date"] = row.AsOfDate.Format(time.DateOnly)
	}
	if h.Clocks != nil {
		if c, ok := h.Clocks.ActiveClock(simID); ok {
			out["clock_loaded"] = true
			out["clock_status"] = c.Status
			out["current_trading_day"] = c.CurrentDate.Format(time.DateOnly)
			out["trading_day_index"] = c.CurrentIndex
			out["total_trading_days"] = len(c.TradingDays)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) StartSimulation(w http.ResponseWriter, r *http.Request) {
	if h.Clocks == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "clock registry not configured"})
		return
	}
	simID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid simulation id"})
		return
	}
	if err := h.Clocks.Start(r.Context(), simID); err != nil {
		writeSimErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"detail": "started"})
}

func (h *Handler) PauseSimulation(w http.ResponseWriter, r *http.Request) {
	if h.Clocks == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "clock registry not configured"})
		return
	}
	simID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid simulation id"})
		return
	}
	if err := h.Clocks.Pause(r.Context(), simID); err != nil {
		writeSimErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"detail": "paused"})
}

// TickSimulation advances the virtual calendar by one trading day (Step 7 operational hook).
func (h *Handler) TickSimulation(w http.ResponseWriter, r *http.Request) {
	if h.Clocks == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "clock registry not configured"})
		return
	}
	simID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid simulation id"})
		return
	}
	if err := h.Clocks.Tick(r.Context(), simID); err != nil {
		writeSimErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"detail": "tick"})
}

func (h *Handler) PutSimSpeed(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "DATABASE_URL required"})
		return
	}
	var req struct {
		SimulationID uuid.UUID `json:"simulation_id"`
		Multiplier   float64   `json:"multiplier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid JSON body"})
		return
	}
	if req.SimulationID == uuid.Nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "simulation_id required"})
		return
	}
	if req.Multiplier <= 0 || req.Multiplier > 86400 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "multiplier must be in (0, 86400]"})
		return
	}
	patch, err := json.Marshal(map[string]float64{"tick_speed_multiplier": req.Multiplier})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "encode failed"})
		return
	}
	if err := simulation.MergeConfig(r.Context(), h.DB, req.SimulationID, patch); err != nil {
		if errors.Is(err, simulation.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"detail": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "update failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"simulation_id":         req.SimulationID.String(),
		"tick_speed_multiplier": req.Multiplier,
	})
}

func (h *Handler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "DATABASE_URL required"})
		return
	}
	simID, err := uuid.Parse(chi.URLParam(r, "simId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid simulation id"})
		return
	}
	if _, err := simulation.Get(r.Context(), h.DB, simID); err != nil {
		if errors.Is(err, simulation.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"detail": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "query failed"})
		return
	}
	entries, err := leaderboard.Build(r.Context(), h.DB, simID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "leaderboard failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"simulation_id": simID.String(),
		"entries":       entries,
	})
}
