# AI Agent Trading Simulation Platform — Complete Roadmap

> A platform where AI agents (different LLM models) compete with each other using virtual money by trading real Indian stocks on historical data.

---

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [High-Level Architecture](#2-high-level-architecture)
3. [Tech Stack Decisions](#3-tech-stack-decisions)
4. [Phase 0 — Foundation & Planning](#4-phase-0--foundation--planning-week-1)
5. [Phase 1 — Data Pipeline](#5-phase-1--data-pipeline-week-12)
6. [Phase 2 — Go Market Simulator](#6-phase-2--go-market-simulator-week-24)
7. [Phase 3 — Python Agent Orchestrator](#7-phase-3--python-agent-orchestrator-week-45)
8. [Phase 4 — End-to-End Integration](#8-phase-4--end-to-end-integration-week-6)
9. [Phase 5 — Dashboard](#9-phase-5--dashboard-week-78)
10. [Phase 6 — Polish & Advanced Features](#10-phase-6--polish--advanced-features-week-9)
11. [Summary Timeline](#11-summary-timeline)
12. [Where to Start Monday Morning](#12-where-to-start-monday-morning)

---

## 1. Project Overview

### What We Are Building

A trading simulation platform where:
- Multiple AI agents (Claude, GPT, Gemini, Llama, etc.) each receive virtual capital (e.g. ₹10,00,000)
- Agents trade real NSE/BSE stocks using **historical data** (free, no live feed needed)
- A **Go service** acts as the market simulator — managing the virtual clock, order matching, and portfolio state
- A **Python service** acts as the agent orchestrator — running LLM agents, feeding them market context, and relaying their decisions to the Go service
- A **dashboard** shows a live leaderboard, equity curves, and trade history

### Free Stock Data Sources (Indian Markets)

| Source | Notes |
|---|---|
| **`yfinance`** | Best starting point. Free, no API key, supports NSE tickers (`RELIANCE.NS`). EOD data. |
| **NSE India** | Official site, scrapable with 15–20 min delay. Use `nsepython`. |
| **Alpha Vantage** | Free tier: 25 calls/day. Supports Indian stocks. |
| **Stooq.com** | Free historical OHLCV, no API key needed. |
| **Zerodha Kite** | Free historical data via developer API (one-time dump). |
| **Upstox Developer API** | Free historical data tier available. |

> **Recommendation:** Start with `yfinance`. Easiest to integrate, completely free, covers NSE well.

---

## 2. High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                       Your Platform                         │
│                                                             │
│  ┌──────────────────────┐    ┌───────────────────────────┐  │
│  │     Go Service       │    │     Python Service        │  │
│  │  (Market Simulator)  │◄──►│  (Agent Orchestrator)    │  │
│  │                      │    │                           │  │
│  │  • Stock data store  │    │  • Agent runner loop      │  │
│  │  • Virtual clock     │    │  • LLM API calls          │  │
│  │  • Order matching    │    │  • Prompt management      │  │
│  │  • Portfolio mgmt    │    │  • Strategy plugins       │  │
│  │  • REST API          │    │  • Analytics & reporting  │  │
│  │  • Leaderboard       │    │  • Agent auth mgmt        │  │
│  └──────────────────────┘    └───────────────────────────┘  │
│             ▲                             ▲                  │
│             │                             │                  │
│      ┌──────┴──────┐              ┌───────┴──────┐          │
│      │  PostgreSQL  │              │    Redis      │          │
│      │  (OHLCV +   │              │  (events,     │          │
│      │  portfolios) │              │  agent queue) │          │
│      └─────────────┘              └──────────────┘          │
└─────────────────────────────────────────────────────────────┘
```

### Communication Between Services

```
┌─────────────────────┐                    ┌─────────────────────┐
│   Python (FastAPI)  │                    │      Go Service     │
│  Agent Orchestrator │                    │  Market Simulator   │
│                     │                    │                     │
│  • Place orders     │ ──── REST ───────► │  • Order matching   │
│  • Fetch quotes     │ ◄─── REST ────────  │  • Portfolio mgmt   │
│  • Get portfolio    │                    │  • Leaderboard      │
│                     │                    │                     │
│  • React to ticks   │ ◄─── Redis ──────  │  • Publish sim tick │
│  • Order filled CB  │     Pub/Sub        │  • Publish fills    │
│  • Session end CB   │                    │  • Publish rejects  │
└─────────────────────┘                    └─────────────────────┘
         │                                          │
         └──────────────┬───────────────────────────┘
                        │
                   PostgreSQL
                (shared read access
                 for market data +
                 portfolio state)
```

---

## 3. Tech Stack Decisions

### Why Go for Market Simulator

- **High performance** — concurrent order processing, portfolio calculations
- **Low latency** HTTP/WebSocket servers via goroutines
- **Strong typing** — financial calculations benefit from strict types, avoids float bugs
- **Concurrency** — handle multiple agents placing orders simultaneously with goroutines + channels

### Why Python for Agent Orchestrator

- **LLM SDKs are Python-first** — Anthropic, OpenAI, LangChain all have best Python support
- **Data analysis** — `pandas`, `numpy` for agents doing technical analysis
- **Rapid iteration** — easy to swap models, tweak prompts, add new strategies
- **Async support** — `asyncio` handles multiple agents calling LLMs concurrently

### Why FastAPI over Django

| | FastAPI | Django |
|---|---|---|
| **Purpose** | Lightweight API service | Full-stack web framework |
| **Async support** | Native (`asyncio`) | Bolted on, awkward |
| **LLM calls** | `await anthropic.client.call()` works perfectly | Needs workarounds |
| **ORM** | SQLAlchemy / Tortoise (your choice) | Forces Django ORM |
| **Speed** | One of the fastest Python frameworks | Significantly slower |
| **Overhead** | Minimal | Admin panel, auth, sessions — none needed here |

### Why REST + Redis over gRPC

| | gRPC | REST + Redis |
|---|---|---|
| **Performance** | ~5–10x faster, binary protocol | Slower but fast enough |
| **Complexity** | High — Proto files, codegen, boilerplate | Low — just HTTP |
| **Debugging** | Hard — binary, needs special tools | Easy — `curl`, Postman |
| **Good for** | Microservices at scale, high RPC volume | Service-to-service at sim frequency |

> gRPC is overkill here. Agents call Go maybe **once per simulation tick** (once per trading day). Latency difference is irrelevant. Revisit only if you scale to 100+ concurrent agents running intraday strategies.

### Final Stack

| Layer | Technology |
|---|---|
| **Market Simulator** | Go + Chi router + pgx + go-redis |
| **Agent Orchestrator** | Python + FastAPI + SQLAlchemy + httpx + redis-py |
| **Database** | PostgreSQL (shared) |
| **Events / Cache** | Redis Pub/Sub |
| **Data Ingestion** | Python + `yfinance` (one-time script) |
| **Dashboard** | React + Recharts |
| **Infrastructure** | Docker Compose |

---

## 4. Phase 0 — Foundation & Planning (Week 1)

### Step 1: Define Simulation Rules (Do This on Paper First)

Before writing a single line of code, decide and document:

```
Simulation Parameters:
├── Starting capital per agent        → e.g. ₹10,00,000
├── Stock universe                    → Nifty 50 only (start small)
├── Simulation time unit              → 1 tick = 1 trading day (EOD)
├── Historical data range             → Jan 2022 – Dec 2024 (3 years)
├── Order types to support            → Market orders only (Phase 1)
├── Constraints                       →  Max 20% capital in one stock
│                                        No short selling (Phase 1)
│                                        Max 10 orders per tick
├── Brokerage simulation              → 0.1% per trade (realistic)
└── Win condition / ranking metric    → Final portfolio value + Sharpe ratio
```

> Locking these down early prevents redesigning your engine mid-build.

### Step 2: Set Up Monorepo

```
trading-sim/
├── market-simulator/        # Go service
├── agent-orchestrator/      # Python service
├── infra/
│   ├── docker-compose.yml   # Postgres + Redis + both services
│   └── init.sql             # DB schema
├── docs/
│   ├── rules.md             # Your simulation rules doc
│   └── api-spec.yaml        # OpenAPI spec (write this early)
└── README.md
```

### Step 3: Set Up Infrastructure

```yaml
# infra/docker-compose.yml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: tradingsim
      POSTGRES_USER: admin
      POSTGRES_PASSWORD: secret
    ports: ["5432:5432"]
    volumes: ["pgdata:/var/lib/postgresql/data"]

  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]

  adminer:             # DB UI — very helpful during dev
    image: adminer
    ports: ["8080:8080"]

volumes:
  pgdata:
```

---

## 5. Phase 1 — Data Pipeline (Week 1–2)

**Goal:** Historical NSE stock data sitting in your database, queryable.

### Step 4: Design the Database Schema

```sql
CREATE TABLE stocks (
    symbol          VARCHAR(20) PRIMARY KEY,  -- 'RELIANCE.NS'
    name            VARCHAR(100),
    sector          VARCHAR(50),
    is_active       BOOLEAN DEFAULT TRUE
);

CREATE TABLE ohlcv (
    id              BIGSERIAL PRIMARY KEY,
    symbol          VARCHAR(20) REFERENCES stocks(symbol),
    date            DATE NOT NULL,
    open            NUMERIC(12,4),
    high            NUMERIC(12,4),
    low             NUMERIC(12,4),
    close           NUMERIC(12,4),
    volume          BIGINT,
    UNIQUE(symbol, date)
);
CREATE INDEX idx_ohlcv_symbol_date ON ohlcv(symbol, date);

CREATE TABLE agents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(100),
    model           VARCHAR(50),      -- 'claude-sonnet-4', 'gpt-4o'
    api_key_hash    VARCHAR(255),
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE simulations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(100),
    start_date      DATE,
    end_date        DATE,
    as_of_date      DATE,             -- simulation clock (not SQL current_date — reserved)
    status          VARCHAR(20),      -- 'pending','running','paused','completed'
    config          JSONB,            -- starting capital, rules, etc.
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE portfolios (
    id              BIGSERIAL PRIMARY KEY,
    simulation_id   UUID REFERENCES simulations(id),
    agent_id        UUID REFERENCES agents(id),
    cash            NUMERIC(15,4),
    UNIQUE(simulation_id, agent_id)
);

CREATE TABLE holdings (
    id              BIGSERIAL PRIMARY KEY,
    portfolio_id    BIGINT REFERENCES portfolios(id),
    symbol          VARCHAR(20),
    quantity        INT,
    avg_buy_price   NUMERIC(12,4),
    UNIQUE(portfolio_id, symbol)
);

CREATE TABLE orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    simulation_id   UUID REFERENCES simulations(id),
    agent_id        UUID REFERENCES agents(id),
    symbol          VARCHAR(20),
    order_type      VARCHAR(10),      -- 'market', 'limit'
    side            VARCHAR(4),       -- 'buy', 'sell'
    quantity        INT,
    price           NUMERIC(12,4),    -- null for market orders
    status          VARCHAR(10),      -- 'pending','filled','rejected','cancelled'
    filled_price    NUMERIC(12,4),
    filled_at       DATE,
    rejection_reason TEXT,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE portfolio_snapshots (   -- daily EOD snapshot per agent
    id              BIGSERIAL PRIMARY KEY,
    simulation_id   UUID REFERENCES simulations(id),
    agent_id        UUID REFERENCES agents(id),
    date            DATE,
    total_value     NUMERIC(15,4),   -- cash + market value of holdings
    cash            NUMERIC(15,4),
    invested_value  NUMERIC(15,4),
    UNIQUE(simulation_id, agent_id, date)
);
```

### Step 5: Build the Data Ingestion Script (Python, One-Time)

```
agent-orchestrator/
└── scripts/
    └── ingest_data.py
```

```python
# scripts/ingest_data.py
import yfinance as yf
import psycopg2

NIFTY_50 = [
    "RELIANCE.NS", "TCS.NS", "HDFCBANK.NS", "INFY.NS", "ICICIBANK.NS",
    "HINDUNILVR.NS", "SBIN.NS", "BHARTIARTL.NS", "ITC.NS", "KOTAKBANK.NS",
    "LT.NS", "AXISBANK.NS", "BAJFINANCE.NS", "MARUTI.NS", "SUNPHARMA.NS",
    "TITAN.NS", "ULTRACEMCO.NS", "NESTLEIND.NS", "WIPRO.NS", "HCLTECH.NS",
    # ... add remaining 30
]

def ingest():
    conn = psycopg2.connect("postgresql://admin:secret@localhost/tradingsim")
    cur = conn.cursor()

    for ticker in NIFTY_50:
        print(f"Fetching {ticker}...")
        df = yf.download(ticker, start="2022-01-01", end="2024-12-31", interval="1d")

        for date_idx, row in df.iterrows():
            cur.execute("""
                INSERT INTO ohlcv (symbol, date, open, high, low, close, volume)
                VALUES (%s, %s, %s, %s, %s, %s, %s)
                ON CONFLICT (symbol, date) DO NOTHING
            """, (ticker, date_idx.date(), float(row['Open']), float(row['High']),
                  float(row['Low']), float(row['Close']), int(row['Volume'])))

        conn.commit()
        print(f"  ✓ {ticker} done")

    cur.close()
    conn.close()

if __name__ == "__main__":
    ingest()
```

> **Milestone ✓** Run the script, verify data in DB via Adminer (localhost:8080). You should have ~700 trading days × 50 stocks = ~35,000 rows.

---

## 6. Phase 2 — Go Market Simulator (Week 2–4)

**Goal:** A running Go service that manages simulation state, processes orders, and exposes a REST API.

### Step 6: Initialize Go Project

```bash
cd market-simulator
go mod init github.com/yourname/trading-sim/market-simulator
go get github.com/go-chi/chi/v5          # Router
go get github.com/jackc/pgx/v5           # Postgres driver
go get github.com/redis/go-redis/v9      # Redis client
go get github.com/google/uuid            # UUIDs
```

### Project Structure

```
market-simulator/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── clock/
│   │   └── clock.go         # Virtual simulation clock
│   ├── market/
│   │   ├── data.go          # OHLCV fetching & caching
│   │   └── quote.go         # Current price provider
│   ├── orderbook/
│   │   ├── engine.go        # Order matching logic
│   │   ├── order.go         # Order types (market, limit, SL)
│   │   └── queue.go         # Pending order queue
│   ├── portfolio/
│   │   ├── manager.go       # Holdings, cash, P&L tracking
│   │   └── metrics.go       # Sharpe, drawdown, returns
│   ├── api/
│   │   ├── routes.go        # REST route definitions
│   │   ├── handlers.go      # Request handlers
│   │   └── ws.go            # WebSocket for live events
│   └── db/
│       └── postgres.go      # DB connection & queries
├── pkg/
│   └── models/              # Shared structs (Order, Trade, etc.)
└── go.mod
```

### Step 7: Build the Simulation Clock

```go
// internal/clock/clock.go
type SimClock struct {
    SimulationID  uuid.UUID
    CurrentDate   time.Time
    TradingDays   []time.Time   // pre-computed from OHLCV data
    CurrentIndex  int
    Status        string        // "running", "paused", "completed"
    redis         *redis.Client
}

func (c *SimClock) Tick() error {
    if c.CurrentIndex >= len(c.TradingDays)-1 {
        c.Status = "completed"
        c.publishEvent("sim.completed", nil)
        return nil
    }
    c.CurrentIndex++
    c.CurrentDate = c.TradingDays[c.CurrentIndex]

    // Publish tick event — Python agents wake up on this
    c.publishEvent("sim.tick", map[string]any{
        "simulation_id": c.SimulationID,
        "date":          c.CurrentDate.Format("2006-01-02"),
    })
    return nil
}
```

### Step 8: Build the Order Matching Engine

Order processing flow per tick:

```
1. Simulation clock ticks to date D
2. Load all PENDING orders for this simulation
3. For each order:
   a. Fetch OHLCV for date D for that symbol
   b. Market order  → fill at Open price of date D
   c. Limit buy     → fill if Low ≤ limit price (fill at limit price)
   d. Limit sell    → fill if High ≥ limit price
   e. Validate: enough cash? enough shares?
   f. Update portfolio holdings + cash
   g. Mark order as filled/rejected
4. Take portfolio snapshot (EOD values)
5. Publish filled/rejected events per agent to Redis
```

### Step 9: REST API Endpoints

```
# Priority 1 — Agents need these
POST /api/v1/orders                    Place an order
GET  /api/v1/portfolio/:agentId        Holdings, cash, P&L
GET  /api/v1/market/quote/:symbol      Current simulated price

# Priority 2 — Useful but not blocking
GET  /api/v1/market/ohlcv/:symbol      Historical candles (?days=20)
GET  /api/v1/orders/:agentId           Order history
DEL  /api/v1/orders/:orderId           Cancel pending order

# Priority 3 — Admin / ops
POST /api/v1/simulations               Create new simulation
POST /api/v1/simulations/:id/start     Start the clock
POST /api/v1/simulations/:id/pause     Pause the clock
PUT  /api/v1/sim/speed                 { "multiplier": 10 }
GET  /api/v1/leaderboard/:simId        Current rankings
```

### Step 10: Add Middleware & Validation

```
Before moving to Python service, harden the Go API:
├── Agent API key auth middleware (check against DB)
├── Rate limiting per agent (100 requests/min max)
├── Order validation (no negative qty, valid symbol, etc.)
├── Request logging (you'll need this to debug agent behavior)
└── Graceful shutdown
```

> **Milestone ✓** Test the full Go service with `curl`. Place a market order, advance the clock one tick, verify the order got filled and portfolio updated correctly.

---

## 7. Phase 3 — Python Agent Orchestrator (Week 4–5)

**Goal:** Python service that runs AI agents, feeds them context, collects their decisions, and places orders via the Go API.

### Step 11: Initialize Python Project

```bash
cd agent-orchestrator
python -m venv venv
source venv/bin/activate

pip install fastapi uvicorn httpx redis sqlalchemy \
            asyncpg anthropic openai python-dotenv \
            pandas numpy pydantic
```

### Project Structure

```
agent-orchestrator/
├── main.py
├── orchestrator/
│   ├── runner.py            # Main agent loop controller
│   ├── scheduler.py         # When each agent gets to act
│   └── event_listener.py    # Listens to Go service events via Redis
├── agents/
│   ├── base_agent.py        # Abstract agent class
│   ├── claude_agent.py      # Anthropic Claude
│   ├── gpt_agent.py         # OpenAI GPT
│   ├── gemini_agent.py      # Google Gemini
│   └── custom_agent.py      # Rule-based / TA-based agent
├── market_client/
│   └── client.py            # HTTP client to call Go service
├── prompts/
│   ├── system.py            # Base trading system prompt
│   └── templates.py         # Context injection templates
├── analytics/
│   └── reporter.py          # Performance reports per agent
├── scripts/
│   └── ingest_data.py       # One-time data ingestion
└── config/
    └── agents.yaml          # Agent configs (model, capital, etc.)
```

### Step 12: Build the Market Client

```python
# market_client/client.py
import httpx
from pydantic import BaseModel

class Order(BaseModel):
    symbol: str
    side: str           # 'buy' or 'sell'
    order_type: str     # 'market' or 'limit'
    quantity: int
    price: float | None = None

class MarketClient:
    def __init__(self, base_url: str, agent_id: str, api_key: str):
        self.base_url = base_url
        self.agent_id = agent_id
        self.headers = {"X-Agent-ID": agent_id, "X-API-Key": api_key}

    async def get_portfolio(self) -> dict:
        async with httpx.AsyncClient() as client:
            r = await client.get(
                f"{self.base_url}/api/v1/portfolio/{self.agent_id}",
                headers=self.headers
            )
            return r.json()

    async def get_ohlcv(self, symbol: str, days: int = 30) -> list[dict]:
        async with httpx.AsyncClient() as client:
            r = await client.get(
                f"{self.base_url}/api/v1/market/ohlcv/{symbol}",
                params={"days": days},
                headers=self.headers
            )
            return r.json()

    async def place_order(self, sim_id: str, order: Order) -> dict:
        async with httpx.AsyncClient() as client:
            r = await client.post(
                f"{self.base_url}/api/v1/orders",
                json={"simulation_id": sim_id, **order.model_dump()},
                headers=self.headers
            )
            return r.json()
```

### Step 13: Build the Base Agent + LLM Agents

```python
# agents/base_agent.py
from abc import ABC, abstractmethod

NIFTY_50 = ["RELIANCE.NS", "TCS.NS", "HDFCBANK.NS", ...]  # full list

class BaseAgent(ABC):
    def __init__(self, agent_id: str, sim_id: str, config: dict):
        self.agent_id = agent_id
        self.sim_id = sim_id
        self.client = MarketClient(
            config["go_service_url"], agent_id, config["api_key"]
        )

    async def build_context(self, current_date: str) -> str:
        portfolio = await self.client.get_portfolio()

        market_data = {}
        for symbol in NIFTY_50[:10]:   # start with top 10
            market_data[symbol] = await self.client.get_ohlcv(symbol, days=20)

        return f"""
        Date: {current_date}

        Your Portfolio:
        - Cash: ₹{portfolio['cash']:,.2f}
        - Holdings: {portfolio['holdings']}
        - Total Value: ₹{portfolio['total_value']:,.2f}
        - P&L: {portfolio['pnl_pct']:.2f}%

        Market Data (last 20 days OHLCV):
        {market_data}

        Rules:
        - Max 20% of capital in any single stock
        - Only BUY and SELL market orders
        - You can place up to 10 orders this turn
        - Return ONLY a JSON array of orders
        """

    @abstractmethod
    async def decide(self, context: str) -> list[Order]:
        pass

    async def run_turn(self, current_date: str):
        context = await self.build_context(current_date)
        orders = await self.decide(context)
        for order in orders:
            result = await self.client.place_order(self.sim_id, order)
            print(f"Agent {self.agent_id}: {order} → {result['status']}")
```

```python
# agents/claude_agent.py
import anthropic, json

class ClaudeAgent(BaseAgent):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.llm = anthropic.Anthropic()

    async def decide(self, context: str) -> list[Order]:
        response = self.llm.messages.create(
            model="claude-sonnet-4-20250514",
            max_tokens=1000,
            system="""You are a stock trading agent competing to maximize
                      portfolio returns. Analyze the market data and portfolio,
                      then return ONLY a valid JSON array of orders.
                      Example: [{"symbol":"TCS.NS","side":"buy","order_type":"market","quantity":5}]
                      Return [] if you want to hold.""",
            messages=[{"role": "user", "content": context}]
        )

        try:
            raw = response.content[0].text.strip()
            orders_data = json.loads(raw)
            return [Order(**o) for o in orders_data]
        except (json.JSONDecodeError, Exception) as e:
            print(f"Claude parse error: {e}")
            return []    # Safe fallback — do nothing this turn
```

### Step 14: Build the Orchestrator (Redis Event Loop)

> **Implementation note:** The Go market-simulator publishes JSON on Redis channel **`sim.events`** with an **`event`** field (`sim.tick`, `sim.completed`, `sim.started`, …). The Python `SimulationRunner` subscribes to that channel (not separate `sim.tick` / `sim.completed` channel names).

```python
# orchestrator/runner.py
import redis.asyncio as redis
import asyncio, json

class SimulationRunner:
    def __init__(self, agents: list):
        self.agents = {a.agent_id: a for a in agents}
        self.redis = redis.Redis(host='localhost', port=6379)

    async def start(self):
        pubsub = self.redis.pubsub()
        await pubsub.subscribe("sim.tick", "sim.completed")
        print("Orchestrator listening for simulation events...")

        async for message in pubsub.listen():
            if message["type"] != "message":
                continue

            channel = message["channel"].decode()
            data = json.loads(message["data"])

            if channel == "sim.tick":
                await self.handle_tick(data)
            elif channel == "sim.completed":
                await self.handle_completion(data)

    async def handle_tick(self, data: dict):
        current_date = data["date"]
        print(f"\n=== Tick: {current_date} ===")

        # Run all agents concurrently on each tick
        await asyncio.gather(*[
            agent.run_turn(current_date)
            for agent in self.agents.values()
        ])
        print(f"All agents done for {current_date}")
```

> **Milestone ✓** Run a 5-day simulation with one Claude agent. Verify it receives market context, returns orders, and Go processes them correctly.

---

## 8. Phase 4 — End-to-End Integration (Week 6)

**Goal:** All pieces working together reliably.

### Step 15: Integration Testing Checklist

> **Implemented in-repo:** Opt-in pytest module `agent-orchestrator/tests/integration/test_phase4_integration.py`.
> Export `RUN_PHASE4_INTEGRATION=1`, set `MARKET_SIMULATOR_URL` + `REDIS_URL`, ensure Postgres holds OHLCV for the simulation window.
> The harness boots **three** agents (Ollama + two `custom`), asserts Redis `sim.events` ticks, leaderboard JSON, and end-of-sim **`completed`**.
> For one real LLM **`run_turn`**, add `OLLAMA_CHAT_INTEGRATION=1`, run `ollama serve`, and **`ollama pull gemma4:latest`** (or set `OLLAMA_MODEL`).

```
□ Create a simulation via API
□ Register 3 agents (Claude, GPT, rule-based)
□ Start simulation — clock begins ticking
□ Python receives sim.tick via Redis
□ All 3 agents fetch context concurrently
□ Agents return orders (JSON parsed correctly)
□ Go validates and fills/rejects orders
□ Portfolio snapshots saved after each tick
□ Leaderboard reflects correct rankings
□ Simulation completes at end date
□ Final P&L report generated
```

### Step 16: Error Handling & Resilience

```
Common failure modes to handle:
├── LLM returns malformed JSON         → fallback to empty orders []
├── LLM API timeout                    → retry once, then skip turn
├── Agent places order for invalid symbol → Go rejects, log reason
├── Agent tries to buy more than cash  → Go rejects gracefully
├── Redis connection drops             → Python reconnects with backoff
└── Go service restarts mid-sim        → resume from last saved state
```

---

## 9. Phase 5 — Dashboard (Week 7–8)

**Goal:** A UI to watch agents compete in real time.

### Step 17: React Dashboard Pages

```
Pages:
├── /simulations             List + create simulations
├── /simulation/:id          Live view of running simulation
│   ├── Leaderboard table    Updates on each tick
│   ├── Equity curves        Portfolio value over time per agent
│   └── Live order feed      Last 20 orders across all agents
├── /agent/:id               Individual agent deep-dive
│   ├── Holdings pie chart
│   ├── Trade history table
│   └── Performance metrics (Sharpe, max drawdown, win rate)
└── /compare                 Side-by-side agent comparison
```

### Step 18: Add WebSocket Feed from Go

```go
// Go pushes live updates to dashboard via WebSocket
// Events to stream:
{ type: "tick",      data: { date, leaderboard } }
{ type: "order",     data: { agent, symbol, side, qty, status } }
{ type: "completed", data: { final_rankings, stats } }
```

---

## 10. Phase 6 — Polish & Advanced Features (Week 9+)

### Step 19: Add More Agent Strategies

```
Agent types to add:
├── Momentum agent       Buys stocks trending up last 10 days
├── Mean reversion       Buys oversold stocks (RSI based)
├── Sector rotation      Rotates capital between sectors
├── Index hugger         Mirrors Nifty 50 weights (baseline/control)
└── Multi-LLM debate     Two LLMs argue before deciding (fun experiment)
```

### Step 20: Add Realistic Market Constraints

```
Make simulation more realistic:
├── Circuit breakers     Stock halts if price moves >10% in a day
├── Liquidity limits     Can't buy more than 1% of daily volume
├── Corporate actions    Handle stock splits, dividends
├── Transaction costs    STT, exchange fees, GST on brokerage
└── Market impact        Large orders move price slightly
```

### Step 21: Performance Metrics to Track Per Agent

| Metric | Description |
|---|---|
| **Total Return** | Final portfolio value vs starting capital |
| **Sharpe Ratio** | Risk-adjusted return (higher is better) |
| **Max Drawdown** | Largest peak-to-trough decline |
| **Win Rate** | % of trades that were profitable |
| **Avg Holding Period** | How long agent holds a stock on average |
| **Sector Exposure** | Which sectors the agent prefers |
| **Turnover Rate** | How often agent trades (activity level) |

---

## 11. Summary Timeline

| Week | Phase | Goal |
|---|---|---|
| 1 | Phase 0 | Rules doc, monorepo, Docker, DB schema |
| 1–2 | Phase 1 | Data ingestion — 50 stocks, 3 years into Postgres |
| 2–4 | Phase 2 | Go service — clock, order engine, REST API |
| 4–5 | Phase 3 | Python service — agents, orchestrator, Redis loop |
| 6 | Phase 4 | End-to-end integration + error handling |
| 7–8 | Phase 5 | React dashboard |
| 9+ | Phase 6 | More agents, realism, experiments |

---

## 12. Where to Start Monday Morning

```
Day 1   git init monorepo, write docker-compose.yml, run Postgres + Redis
Day 2   Write DB schema (init.sql), run it, verify tables in Adminer
Day 3   Write + run data ingestion script, verify ~35k rows in ohlcv table
Day 4   Start Go project, connect to DB, write /health and /quote endpoints
Day 5   Build simulation clock + first tick test
```

---

## Notes & Open Questions

> Use this section to track decisions you haven't made yet.

- [ ] Stock universe: Nifty 50 only, or Nifty 500?
- [ ] Simulation speed: 1 tick/second (demo mode) or instant batch replay?
- [ ] Intraday support: EOD only for now, or 5-min candles later?
- [ ] Agent constraints: Should agents know what other agents are doing?
- [ ] Hosting: Local dev only, or deploy to cloud (Fly.io / Railway)?
- [ ] LLM cost management: Set a per-agent token budget per simulation?

---

*Last updated: May 2026*