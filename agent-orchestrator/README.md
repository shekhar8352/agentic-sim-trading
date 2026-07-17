# Agent orchestrator (Python / FastAPI)

Runs AI agents, manages prompts, and talks to the Go **market-simulator** service. See `docs/rroadmap.md` (Phase 3–4, Steps 11–16) and `docs/rules.md`.

## Project layout (Step 11)

```
agent-orchestrator/
├── app/                    # FastAPI app, settings, YAML config loader
├── orchestrator/           # SimulationRunner (Redis), AgentRunner, scheduler, listener
├── agents/                 # Base + Claude / GPT / Gemini / Ollama / custom
├── market_client/          # HTTP client for Go API (Step 12)
├── prompts/                # System prompt + context templates
├── analytics/              # Performance reporting scaffold
├── config/agents.yaml      # Agent definitions (ids/keys after registration)
├── scripts/ingest_data.py  # OHLCV ingestion (Step 5)
└── tests/
    └── integration/        # Step 15 — RUN_PHASE4_INTEGRATION=1
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
| `OLLAMA_BASE_URL` / `OLLAMA_MODEL` | Local Ollama (`http://127.0.0.1:11434`, model `gemma4:latest`) |

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

## Redis orchestrator (Step 14)

The Go market-simulator publishes JSON on Redis channel **`sim.events`** (field **`event`**: `sim.tick`, `sim.completed`, …). `SimulationRunner` subscribes, filters by `simulation_id` from `config/agents.yaml`, and runs **`asyncio.gather`** over all agents on each `sim.tick`. When `sim.completed` arrives for that simulation, the runner stops (configurable via `stop_on_completed`).

From `agent-orchestrator/` after setting `REDIS_URL`, `simulation_id`, and agent credentials in `config/agents.yaml`:

```bash
python -m orchestrator
```

Use `AgentRunner` when you want a **sequential** manual loop (tests); use `SimulationRunner` for the live Redis-driven loop.

## Phase 4 — Step 15 (integration + Ollama)

Automated checklist lives in **`tests/integration/test_phase4_integration.py`**. Defaults assume **Postgres populated with OHLCV**, Redis, and the simulator on `MARKET_SIMULATOR_URL`.

- Bootstrap creates one simulation plus **three** registered agents: **Ollama** + two **`custom`** momentum agents (`tests/integration/harness.py`).
- Verifies **`sim.tick`** on Redis channel **`sim.events`**, leaderboard JSON, driving ticks until **`completed`**.
- For a **live** LLM call through your stack (`ollama pull` your tag first):

```bash
export RUN_PHASE4_INTEGRATION=1 OLLAMA_CHAT_INTEGRATION=1
export MARKET_SIMULATOR_URL=http://127.0.0.1:8070 REDIS_URL=redis://127.0.0.1:6379/0
pytest tests/integration/test_phase4_integration.py -v -k leaderboard
```

`config/agents.yaml` can declare an Ollama row with **`provider: ollama`** and **`model`** (for example **`gemma4:latest`**).

## Step 16 — error handling & resilience

Failure modes handled in the orchestrator:

| Failure | Behavior |
|---------|----------|
| LLM returns malformed JSON | Parse fallback → hold (`[]`); warning logged |
| LLM API timeout | Retry once, then skip turn |
| Invalid symbol / insufficient cash | Go rejects order; reason logged via `APIError.detail` |
| Go unavailable during context fetch | Use cached portfolio/OHLCV if available; else skip turn |
| Redis disconnect | Exponential backoff reconnect on `sim.events` |
| Go restart mid-sim | `sim.resumed` logged; next `sim.tick` continues from Go saved state |

### Agents (Step 13)

| Class | Provider | Notes |
|-------|----------|-------|
| `ClaudeAgent` | `claude` | Async Anthropic API |
| `GPTAgent` | `gpt` / `openai` | Async OpenAI API |
| `GeminiAgent` | `gemini` | Requires `pip install -e '.[llm]'` |
| `OllamaAgent` | `ollama` | HTTP `/api/chat` on `OLLAMA_BASE_URL`; default model `gemma4:latest` |
| `CustomAgent` | `custom` | Momentum rule-based baseline |

`BaseAgent.run_turn(date)` builds context (portfolio + top-10 Nifty OHLCV), calls `decide()`, and submits up to 10 orders via `MarketClient`.

### Multi-agent trading desk

LLM providers (**claude / gpt / gemini / ollama**) default to a **team** on each tick — one Go portfolio, several role prompts:

| Role | Job |
|------|-----|
| **Analyst** | Reads OHLCV trends; writes a market briefing (no orders) |
| **Risk Officer** | Sets cash / concentration constraints; vetoes unsafe ideas |
| **Strategist** | Proposes a short prioritized trade plan |
| **Head Trader** | Synthesizes teammate reports and emits the final JSON orders |

Set `team_mode: false` on an agent entry (or uncheck “Multi-agent desk” in the dashboard) for the legacy single-prompt trader. Customize roles with `team.roles` / `team_roles` (must include `head`).

```python
from agents import create_agent
from app.config_loader import load_agents_config

cfg = load_agents_config("config/agents.yaml")
entry = cfg.agents[0]
agent = create_agent(entry.agent_id, cfg.simulation_id, entry, cfg.go_service_url)
await agent.run_turn("2024-01-15")
```

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

Pass `simulation_id` at construction (as `BaseAgent` does) or per call. Errors raise `APIError` (HTTP rejection), `TransportError` (network/timeout), or `ConfigurationError`.

```python
from market_client import MarketClient, Order

async with MarketClient(base_url, agent_id, api_key, simulation_id=sim_id) as client:
    portfolio = await client.get_portfolio()
    await client.place_order(Order(symbol="TCS.NS", side="buy", quantity=1))
```
