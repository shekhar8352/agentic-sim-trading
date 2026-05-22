# Agent orchestrator (Python / FastAPI)

Runs AI agents, manages prompts, and talks to the Go **market-simulator** service. See `docs/rroadmap.md` (Phase 3, Steps 11–14) and `docs/rules.md`.

## Project layout (Step 11)

```
agent-orchestrator/
├── app/                    # FastAPI app, settings, YAML config loader
├── orchestrator/           # Runner, scheduler, Redis event listener
├── agents/                 # Base + Claude / GPT / Gemini / custom stubs
├── market_client/          # HTTP client for Go API (Step 12)
├── prompts/                # System prompt + context templates
├── analytics/              # Performance reporting scaffold
├── config/agents.yaml      # Agent definitions (ids/keys after registration)
├── scripts/ingest_data.py  # OHLCV ingestion (Step 5)
└── tests/
```

## Run locally

From **repository root**:

```bash
make install-orchestrator   # pip install -e ".[dev]"
make dev-orchestrator       # :8071
```

Or from this directory:

```bash
python -m venv .venv
source .venv/bin/activate
cp .env.example .env        # optional
pip install -e ".[dev]"
uvicorn app.main:app --reload --host 0.0.0.0 --port 8071
```

Environment variables (see `.env.example`):

| Variable | Purpose |
|----------|---------|
| `MARKET_SIMULATOR_URL` | Go service base URL (default `http://localhost:8070`) |
| `REDIS_URL` | Simulation events (orchestrator listener) |
| `DATABASE_URL` | Optional async SQLAlchemy URL for future persistence |
| `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` / `GOOGLE_API_KEY` | LLM providers (Step 13) |

Optional extras:

```bash
pip install -e ".[llm]"     # google-generativeai for GeminiAgent
pip install -e ".[ingest]"  # yfinance + psycopg2 for scripts/ingest_data.py
```

### Register agents with the simulator

1. Create a simulation and register an agent via the Go API (`POST /api/v1/simulations/{id}/agents`).
2. Copy returned `agent_id` and `api_key` into `config/agents.yaml`.
3. Use those credentials in `MarketClient` / agent classes (Step 12–13).

### Load historical OHLCV (roadmap Step 5)

Requires Postgres (`infra/docker-compose.data.yml` or full stack). Install ingestion extras:

```bash
make install-orchestrator-ingest
export DATABASE_URL='postgresql://admin:secret@localhost:5432/tradingsim'
python scripts/ingest_data.py
```

## Docker

Built by `infra/docker-compose.yml` as service `agent-orchestrator`. From repo root: `make up`.

## Next steps

- **Step 13**: Implement `build_context` / `decide` on LLM agents (uses `MarketClient.get_portfolio`, `get_ohlcv`).
- **Step 14**: Wire `AgentRunner` + `SimulationEventListener` to simulation ticks.

### Market client (Step 12)

`market_client.MarketClient` wraps all **agent-authenticated** Go routes:

| Method | Go endpoint |
|--------|-------------|
| `get_portfolio()` | `GET /api/v1/portfolio/{agent_id}` |
| `get_quote(symbol)` | `GET /api/v1/market/quote/{symbol}` |
| `get_ohlcv(symbol, days=30)` | `GET /api/v1/market/ohlcv/{symbol}` |
| `list_orders(limit=100)` | `GET /api/v1/orders/{agent_id}` |
| `place_order(order)` | `POST /api/v1/orders` |
| `cancel_order(order_id)` | `DELETE /api/v1/orders/{order_id}` |
| `health()` | `GET /health` (no auth) |

Pass `simulation_id` at construction (as `BaseAgent` does) or per call. Errors raise `APIError` with `status_code` and `detail` from the simulator JSON body.

```python
from market_client import MarketClient, Order

async with MarketClient(base_url, agent_id, api_key, simulation_id=sim_id) as client:
    portfolio = await client.get_portfolio()
    await client.place_order(Order(symbol="TCS.NS", side="buy", quantity=1))
```
