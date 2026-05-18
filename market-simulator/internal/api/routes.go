package api

import (
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter registers roadmap Step 9 REST routes with Step 10 agent auth and rate limits.
func NewRouter(h *Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(RequestLog)

	r.Get("/health", h.Health)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/simulations", h.CreateSimulation)
		r.Get("/simulations/{id}", h.GetSimulation)
		r.Post("/simulations/{id}/start", h.StartSimulation)
		r.Post("/simulations/{id}/pause", h.PauseSimulation)
		r.Post("/simulations/{id}/tick", h.TickSimulation)
		r.Post("/simulations/{id}/agents", h.RegisterAgent)

		r.Put("/sim/speed", h.PutSimSpeed)

		r.Get("/leaderboard/{simId}", h.GetLeaderboard)

		r.Get("/live/stream", h.LiveEvents)

		r.Group(func(r chi.Router) {
			r.Use(AgentAuth(h.DB))
			r.Use(RateLimitPerAgent(100, time.Minute))
			r.Post("/orders", h.PlaceOrder)
			r.Get("/portfolio/{agentId}", h.GetPortfolio)
			r.Get("/market/quote/{symbol}", h.GetQuote)
			r.Get("/market/ohlcv/{symbol}", h.GetOHLCV)
			r.Get("/orders/{agentId}", h.ListOrders)
			r.Delete("/orders/{orderId}", h.CancelOrder)
		})
	})
	return r
}
