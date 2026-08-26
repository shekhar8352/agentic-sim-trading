package api

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/agentic-sim-trading/market-simulator/internal/agents"
	"github.com/agentic-sim-trading/market-simulator/internal/portfolio"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AgentAuth verifies X-Agent-ID + X-API-Key against agents.api_key_hash (Step 10).
func AgentAuth(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if SkipAgentAuth() {
				idStr := strings.TrimSpace(r.Header.Get("X-Agent-ID"))
				if idStr != "" {
					if id, err := uuid.Parse(idStr); err == nil {
						r = r.WithContext(ContextWithAgentID(r.Context(), id))
					}
				}
				next.ServeHTTP(w, r)
				return
			}
			if pool == nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"detail": "database required for agent authentication"})
				return
			}
			idStr := strings.TrimSpace(r.Header.Get("X-Agent-ID"))
			apiKey := strings.TrimSpace(r.Header.Get("X-API-Key"))
			agentID, err := uuid.Parse(idStr)
			if err != nil || apiKey == "" {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "X-Agent-ID and X-API-Key headers required"})
				return
			}

			switch credErr := agents.ValidateCredentials(r.Context(), pool, agentID, apiKey); {
			case errors.Is(credErr, agents.ErrAgentNotFound):
				writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "unknown agent"})
			case errors.Is(credErr, agents.ErrAPIKeyNotConfigured):
				writeJSON(w, http.StatusForbidden, map[string]string{"detail": "api key not configured"})
			case credErr != nil:
				writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "invalid credentials"})
			default:
				next.ServeHTTP(w, r.WithContext(ContextWithAgentID(r.Context(), agentID)))
			}
		})
	}
}

type slidingLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	window time.Duration
	max    int
}

func newSlidingLimiter(max int, window time.Duration) *slidingLimiter {
	if max <= 0 {
		max = 100
	}
	if window <= 0 {
		window = time.Minute
	}
	return &slidingLimiter{
		hits:   make(map[string][]time.Time),
		max:    max,
		window: window,
	}
}

func (l *slidingLimiter) allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	var kept []time.Time
	for _, t := range l.hits[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.max {
		l.hits[key] = kept
		return false
	}
	kept = append(kept, now)
	l.hits[key] = kept
	return true
}

func rateLimitKey(r *http.Request) string {
	if id, ok := AgentIDFromContext(r.Context()); ok {
		return "agent:" + id.String()
	}
	return "ip:" + strings.TrimSpace(r.RemoteAddr)
}

// RateLimitPerTick enforces 100 API calls per agent per simulation tick (rules §11.4)
// and tracks consecutive 4xx for disqualification (rules §12).
func (h *Handler) RateLimitPerTick(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agentID, ok := AgentIDFromContext(r.Context())
		if ok && h.Clocks != nil {
			simID := uuid.Nil
			if h.Portfolio != nil {
				if id, err := h.Portfolio.SimulationIDForAgent(r.Context(), agentID); err == nil {
					simID = id
				}
			}
			if sid := strings.TrimSpace(r.URL.Query().Get("simulation_id")); sid != "" {
				if parsed, err := uuid.Parse(sid); err == nil {
					simID = parsed
				}
			}
			if simID != uuid.Nil && !h.Clocks.AllowAPICall(simID, agentID) {
				writeJSON(w, http.StatusTooManyRequests, map[string]string{"detail": "rate limit exceeded (100 requests per tick)"})
				return
			}
		}

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		if !ok || h.Portfolio == nil {
			return
		}
		st := ww.Status()
		if st >= 400 && st < 500 && st != http.StatusUnauthorized && st != http.StatusTooManyRequests {
			simID, n, err := h.Portfolio.Increment4xx(r.Context(), agentID)
			if err == nil && n >= portfolio.Consecutive4xxDQ && simID != uuid.Nil {
				_ = h.Portfolio.SetDisqualified(r.Context(), simID, agentID)
			}
			return
		}
		if st >= 200 && st < 400 {
			_ = h.Portfolio.Reset4xx(r.Context(), agentID)
		}
	})
}

// RateLimitPerAgent enforces max requests per window keyed by authenticated agent (or IP).
func RateLimitPerAgent(max int, window time.Duration) func(http.Handler) http.Handler {
	lim := newSlidingLimiter(max, window)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := rateLimitKey(r)
			if !lim.allow(key) {
				writeJSON(w, http.StatusTooManyRequests, map[string]string{"detail": "rate limit exceeded (100 requests per minute)"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequestLog logs method, path, status, duration, optional agent id and request id (Step 10).
func RequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		reqID := middleware.GetReqID(r.Context())
		agent := ""
		if id, ok := AgentIDFromContext(r.Context()); ok {
			agent = id.String()
		}
		log.Printf("request method=%s path=%s status=%d dur_ms=%d bytes=%d agent_id=%s req_id=%s remote_ip=%s",
			r.Method, r.URL.Path, ww.Status(), time.Since(start).Milliseconds(), ww.BytesWritten(), agent, reqID, r.RemoteAddr)
	})
}
