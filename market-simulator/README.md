# Market simulator (Go)

REST API and simulation engine for virtual order matching, portfolio state, and time advancement. See `docs/rroadmap.md` (Phase 2 — Step 6+) and `docs/rules.md`.

## Layout

- `cmd/server` — HTTP entrypoint, schema migrate, clock resume
- `internal/clock` — virtual simulation clock, submission window, Redis tick events
- `internal/market` — OHLCV reads / quote provider (as-of clamped)
- `internal/orderbook` — Postgres-backed EOD matching (market, limit, stop)
- `internal/portfolio` — holdings, Day-0 snapshots, DQ, risk metrics
- `internal/fees` — Indian market fee breakdown (rules §8)
- `internal/api` — Chi routes, agent auth, WebSocket live stream
- `internal/db` — Postgres pool + idempotent migrations
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
| `SIMULATOR_SKIP_AGENT_AUTH` | Dev bypass for API key checks |

Without `DATABASE_URL`, `/health` still returns OK; quote/OHLCV endpoints respond `501` with a JSON hint.

## Tick pipeline (rules §4)

1. Clock advances to trading date **D** (first tick processes the start date).
2. Publish `sim.tick` on Redis channel `sim.events`.
3. Agents submit orders during `order_window_seconds` (default 3s; `0` = batch / immediate match).
4. Window closes; pending orders for **D** fill at that day's open (limits/stops may defer).
5. EOD snapshots; publish `sim.tick.processed`.

`tick_speed_multiplier` scales both the auto-tick interval and the order window.

## Simulation clock

Trading days are derived from **distinct `ohlcv.date`** values between each simulation’s `start_date` and `end_date`.

| Method | Path | Purpose |
| -------- | ---- | ------- |
| `POST` | `/api/v1/simulations` | Create simulation (`{"name","start_date","end_date","config?"}` — dates `YYYY-MM-DD`). Initial row status: `paused`. |
| `GET` | `/api/v1/simulations/{id}` | DB row + optional in-memory clock (`clock_loaded`, indices). |
| `POST` | `/api/v1/simulations/{id}/start` | Build/resume clock, set `running`, persist `as_of_date`, publish **`sim.started`** or **`sim.resumed`**. |
| `POST` | `/api/v1/simulations/{id}/pause` | Set `paused` (clock stays in memory). |
| `POST` | `/api/v1/simulations/{id}/tick` | Advance / open submission window / match; persist; publish **`sim.tick`** or **`sim.completed`**. |
| `GET` | `/api/v1/live/stream` | WebSocket fan-out of Redis `sim.events` (`?simulation_id=` optional). |

On process restart, simulations with DB status `running` are reloaded automatically.

Redis channel **`sim.events`**: JSON payloads include `"event"` (`sim.started`, `sim.resumed`, `sim.tick`, `sim.tick.processed`, `sim.completed`, `order.filled`, `order.rejected`) plus `simulation_id`, `date`, etc.

**Market data:** optional query `simulation_id` on `GET /api/v1/market/quote/{symbol}` and `GET /api/v1/market/ohlcv/{symbol}` uses that simulation’s current `as_of_date` (from the active clock or Postgres).

Build binary (output in `bin/server`, gitignored):

```bash
make go-build    # from repo root
```

## Docker

Built by `infra/docker-compose.yml` as service `market-simulator`. From repo root: `make up`.
