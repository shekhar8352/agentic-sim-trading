# Agentic sim trading

Monorepo for an **AI agent trading simulation** on historical Indian equities: Go **market-simulator**, Python **agent-orchestrator**, shared **PostgreSQL** + **Redis**, and docs.

Layout (Phase 0):

| Path | Purpose |
|------|--------|
| `market-simulator/` | Go service — virtual clock, matching, portfolios, REST API |
| `agent-orchestrator/` | FastAPI — LLM agents, prompts, calls into the simulator |
| `infra/` | `docker-compose.yml`, `init.sql` |
| `docs/` | `rules.md`, `rroadmap.md`, `api-spec.yaml` |

## Quick start (Docker)

From repo root:

```bash
cd infra
docker compose up --build
```

- **PostgreSQL:** `localhost:5432`, database `tradingsim`, user `admin`, password `secret`
- **Redis:** `localhost:6379`
- **Adminer:** http://localhost:8080 (system: PostgreSQL, server: `postgres`, user/password as above)
- **Market simulator:** http://localhost:8070/health
- **Agent orchestrator:** http://localhost:8071/health

## Local dev (without Docker)

- Go simulator: see `market-simulator/README.md`
- Orchestrator: see `agent-orchestrator/README.md`

## Simulation rules

Authoritative parameters and constraints: **`docs/rules.md`**.
