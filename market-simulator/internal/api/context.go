package api

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey int

const authenticatedAgentKey ctxKey = iota

// ContextWithAgentID attaches the authenticated agent id (after API key verification).
func ContextWithAgentID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, authenticatedAgentKey, id)
}

// AgentIDFromContext returns the agent id set by agent auth middleware.
func AgentIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v := ctx.Value(authenticatedAgentKey)
	if v == nil {
		return uuid.Nil, false
	}
	id, ok := v.(uuid.UUID)
	return id, ok && id != uuid.Nil
}
