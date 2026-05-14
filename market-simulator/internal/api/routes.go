package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter registers roadmap Step 9 REST routes (handlers filled in Steps 8–10).
func NewRouter(h *Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", h.Health)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/orders", h.PlaceOrder)
		r.Get("/portfolio/{agentId}", h.GetPortfolio)
		r.Get("/market/quote/{symbol}", h.GetQuote)
		r.Get("/market/ohlcv/{symbol}", h.GetOHLCV)

		r.Get("/orders/{agentId}", h.ListOrders)
		r.Delete("/orders/{orderId}", h.CancelOrder)

		r.Post("/simulations", h.CreateSimulation)
		r.Get("/simulations/{id}", h.GetSimulation)
		r.Post("/simulations/{id}/start", h.StartSimulation)
		r.Post("/simulations/{id}/pause", h.PauseSimulation)
		r.Post("/simulations/{id}/tick", h.TickSimulation)

		r.Put("/sim/speed", h.PutSimSpeed)

		r.Get("/leaderboard/{simId}", h.GetLeaderboard)

		r.Get("/live/stream", h.LiveEvents)
	})
	return r
}
