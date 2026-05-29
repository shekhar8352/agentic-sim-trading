# Agentic sim trading

Monorepo for an **AI agent trading simulation** on historical Indian equities: Go **market-simulator**, Python **agent-orchestrator**, shared **PostgreSQL** + **Redis**, and docs.

Layout (Phase 0):

| Path | Purpose |
|------|--------|
| `market-simulator/` | Go service — virtual clock, matching, portfolios, REST API |
| `agent-orchestrator/` | FastAPI — LLM agents, prompts, calls into the simulator |
| `dashboard/` | React — live leaderboard, equity curves, agent comparison |
| `infra/` | `docker-compose.yml`, `init.sql` |
| `docs/` | `rules.md`, `rroadmap.md`, `api-spec.yaml` |
| `Makefile` | Repo-root targets: `make help`, `make up`, local dev, Go tooling |

## Makefile

From repo root, `make help` lists targets. Common commands:

| Command | Description |
|---------|-------------|
| `make up` | Start full stack (`docker compose` in `infra/`, with build) |
| `make down` | Stop Compose stack |
| `make dev-simulator` | Run Go simulator locally on `:8070` |
| `make install-orchestrator` then `make dev-orchestrator` | Run FastAPI locally on `:8071` |

## Quick start (Docker)

From repo root:

```bash
make up
```

Equivalent manual command:

```bash
cd infra && docker compose up --build
```

- **PostgreSQL:** `localhost:5432`, database `tradingsim`, user `admin`, password `secret`
- **Redis:** `localhost:6379`
- **Adminer:** http://localhost:8080 (system: PostgreSQL, server: `postgres`, user/password as above)
- **Market simulator:** http://localhost:8070/health
- **Agent orchestrator:** http://localhost:8071/health
- **Dashboard:** http://localhost:5173 (with `make up`)

## Local dev (without Docker)

- `make dev-simulator` — or see `market-simulator/README.md`
- `make install-orchestrator` and `make dev-orchestrator` — or see `agent-orchestrator/README.md`

## Simulation rules

Authoritative parameters and constraints: **`docs/rules.md`**.
