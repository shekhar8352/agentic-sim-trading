# Market simulator (Go)

REST API and simulation engine for virtual order matching, portfolio state, and time advancement. See `docs/rroadmap.md` (Phase 2 — Step 6+) and `docs/rules.md`.

## Layout (roadmap Step 6)

- `cmd/server` — HTTP entrypoint
- `internal/clock` — virtual simulation clock / Redis tick events
- `internal/market` — OHLCV reads / quote provider
- `internal/orderbook` — matching engine scaffolding (Step 8)
- `internal/portfolio` — holdings & metrics helpers
- `internal/api` — Chi routes & handlers
- `internal/db` — Postgres pool (`DATABASE_URL`)
- `internal/redisconn` — optional Redis client (`REDIS_URL`)
- `pkg/models` — shared domain structs

## Run locally

From **repository root**:

```bash
make dev-simulator
```

Or from this directory:

```bash
go run ./cmd/server
```

Default listen address: `:8070`. Override with `LISTEN_ADDR`.

**Environment**

| Variable | Purpose |
| -------- | ------- |
| `DATABASE_URL` | Postgres connection string (enables `/api/v1/market/*`, portfolio lookups). Example: `postgres://admin:secret@localhost:5432/tradingsim` |
| `REDIS_URL` | Redis for simulation pub/sub (clock publishes `sim.events`). Example: `redis://localhost:6379/0` |

Without `DATABASE_URL`, `/health` still returns OK; quote/OHLCV endpoints respond `501` with a JSON hint.

Build binary (output in `bin/server`, gitignored):

```bash
make go-build    # from repo root
```

## Docker

Built by `infra/docker-compose.yml` as service `market-simulator`. From repo root: `make up`.
