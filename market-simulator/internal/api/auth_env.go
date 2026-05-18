package api

import (
	"os"
	"strings"
)

// SkipAgentAuth disables API-key verification when SIMULATOR_SKIP_AGENT_AUTH=true (dev only).
func SkipAgentAuth() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("SIMULATOR_SKIP_AGENT_AUTH")), "true")
}
