package api

import (
	"net/http"

	"github.com/google/uuid"
)

// agentAccessDeniedFromRequest returns true if the caller may not act as claimed (403/401 already written).
// With SIMULATOR_SKIP_AGENT_AUTH=true, enforcement is skipped for local development.
func agentAccessDeniedFromRequest(w http.ResponseWriter, r *http.Request, claimed uuid.UUID) bool {
	if SkipAgentAuth() {
		return false
	}
	authID, ok := AgentIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "missing agent context"})
		return true
	}
	if authID != claimed {
		writeJSON(w, http.StatusForbidden, map[string]string{"detail": "agent id does not match authenticated agent"})
		return true
	}
	return false
}
