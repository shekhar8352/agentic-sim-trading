# Simulation Rules

> This document defines the precise, binding rules for the AI Agent Trading Simulation Platform.
> All services (Go market simulator, Python agent orchestrator) must enforce these rules exactly.
> Any change to this document must be reflected in both services before the next simulation run.

---

## Table of Contents

1. [Capital & Accounts](#1-capital--accounts)
2. [Stock Universe](#2-stock-universe)
3. [Historical Data](#3-historical-data)
4. [Simulation Clock](#4-simulation-clock)
5. [Order Rules](#5-order-rules)
6. [Portfolio Constraints](#6-portfolio-constraints)
7. [Order Execution & Pricing](#7-order-execution--pricing)
8. [Transaction Costs](#8-transaction-costs)
9. [Market Restrictions](#9-market-restrictions)
10. [Ranking & Win Condition](#10-ranking--win-condition)
11. [Agent Behaviour Rules](#11-agent-behaviour-rules)
12. [Disqualification Conditions](#12-disqualification-conditions)
13. [Versioning & Change Log](#13-versioning--change-log)

---

## 1. Capital & Accounts

| Parameter | Value |
|---|---|
| **Starting capital per agent** | ₹10,00,000 (Ten Lakh INR) |
| **Currency** | Indian Rupee (INR) |
| **Capital type** | Virtual (no real money involved) |
| **Capital top-up** | Not allowed during a simulation run |
| **Borrowing / Margin** | Not allowed |
| **Capital shared between agents** | No — each agent has a fully independent portfolio |

### Account Initialisation

When a simulation starts, every registered agent receives:

```
cash_balance    = ₹10,00,000
holdings        = {} (empty)
total_value     = ₹10,00,000
```

The Go service creates a `portfolios` row and an initial `portfolio_snapshots` row for Day 0 (the day before simulation start date) with the above values.

---

## 2. Stock Universe

| Parameter | Value |
|---|---|
| **Universe** | Nifty 50 constituents only |
| **Exchange** | NSE (National Stock Exchange of India) |
| **Ticker format** | `SYMBOL.NS` (e.g. `RELIANCE.NS`, `TCS.NS`) |
| **Total tradeable symbols** | 50 |
| **Universe locked** | Yes — fixed at simulation creation time, no mid-simulation changes |

### Nifty 50 Symbol List

```
ADANIENT.NS    ADANIPORTS.NS  APOLLOHOSP.NS  ASIANPAINT.NS  AXISBANK.NS
BAJAJ-AUTO.NS  BAJFINANCE.NS  BAJAJFINSV.NS  BPCL.NS        BHARTIARTL.NS
BRITANNIA.NS   CIPLA.NS       COALINDIA.NS   DIVISLAB.NS    DRREDDY.NS
EICHERMOT.NS   GRASIM.NS      HCLTECH.NS     HDFCBANK.NS    HDFCLIFE.NS
HEROMOTOCO.NS  HINDALCO.NS    HINDUNILVR.NS  ICICIBANK.NS   ITC.NS
INDUSINDBK.NS  INFY.NS        JSWSTEEL.NS    KOTAKBANK.NS   LT.NS
LTIM.NS        M&M.NS         MARUTI.NS      NESTLEIND.NS   NTPC.NS
ONGC.NS        POWERGRID.NS   RELIANCE.NS    SBILIFE.NS     SBIN.NS
SUNPHARMA.NS   TATACONSUM.NS  TATAMOTORS.NS  TATASTEEL.NS   TCS.NS
TECHM.NS       TITAN.NS       ULTRACEMCO.NS  UPL.NS         WIPRO.NS
```

> **Note:** The Nifty 50 composition used is the one valid as of the simulation start date (3 January 2022). If a stock is replaced mid-index during the data range, the original composition is kept frozen for the simulation. This avoids survivorship bias decisions mid-run.

---

## 3. Historical Data

| Parameter | Value |
|---|---|
| **Data source** | Yahoo Finance (`yfinance` Python library) |
| **Data range** | 1 January 2022 – 31 December 2024 (3 years) |
| **Granularity** | End-of-Day (EOD) — one OHLCV bar per trading day |
| **Fields stored** | `open`, `high`, `low`, `close`, `volume` |
| **Adjusted prices** | Yes — use split/dividend-adjusted prices throughout |
| **Live / real-time data** | Not used — simulation is entirely historical replay |
| **Data currency** | INR (all prices as returned by Yahoo Finance for `.NS` tickers) |

### Trading Day Definition

A trading day is any calendar date for which NSE OHLCV data exists in the database for at least one symbol. Weekends, national holidays, and exchange closure days are automatically excluded because no data exists for them. The list of valid trading days is pre-computed from the `ohlcv` table at simulation creation time and stored as an ordered array in the `simulations` record.

---

## 4. Simulation Clock

| Parameter | Value |
|---|---|
| **Time unit (tick)** | 1 tick = 1 trading day |
| **Clock type** | Virtual — not tied to wall clock time |
| **Simulation start date** | 3 January 2022 (first trading day in dataset) |
| **Simulation end date** | 31 December 2024 (last trading day in dataset) |
| **Total ticks (approx.)** | ~730 trading days |
| **Default auto-tick speed** | 1 tick per 5 seconds (demo/observation mode) |
| **Batch mode** | Supported — replay all ticks instantly with no delay |

### Clock Lifecycle

```
PENDING → RUNNING → PAUSED → RUNNING → ... → COMPLETED
```

| Status | Description |
|---|---|
| **PENDING** | Simulation created, agents registered, clock not yet started |
| **RUNNING** | Clock is ticking day by day, agents are active |
| **PAUSED** | Clock stopped mid-simulation, agents do not act |
| **COMPLETED** | End date reached, no further orders accepted, final snapshots taken |

### Tick Sequence (Per Trading Day)

On each tick, the Go service executes the following steps in order:

```
1. Advance clock to next trading date D
2. Publish sim.tick event to Redis  →  agents begin submitting orders
3. Wait for order submission window (configurable, default 3 seconds in demo mode,
   0 seconds in batch mode)
4. Close order submission for date D
5. Load all PENDING orders for this simulation
6. Process each order (see Section 7 for execution rules)
7. Recalculate each agent's total portfolio value at D's close price
8. Save a portfolio_snapshots row for each agent
9. Publish sim.tick.processed event to Redis
```

### Information Barrier (No Future Peeking)

The Go service must **never return OHLCV data, prices, or any market information for dates beyond `simulation.as_of_date`**. This is enforced at the API query layer on every request. Violating this rule gives agents future information and invalidates the simulation entirely.

---

## 5. Order Rules

### 5.1 Supported Order Types

| Order Type | Phase 1 | Fill Behaviour |
|---|---|---|
| **Market Buy** | ✅ Supported | Fills at `open` price of current tick date |
| **Market Sell** | ✅ Supported | Fills at `open` price of current tick date |
| **Limit Buy** | ❌ Not supported | Planned for a future phase |
| **Limit Sell** | ❌ Not supported | Planned for a future phase |
| **Stop-Loss** | ❌ Not supported | Planned for a future phase |

Agents submitting unsupported order types receive status `rejected` with reason `order_type_not_supported`.

### 5.2 Order Limits Per Tick

| Parameter | Value |
|---|---|
| **Maximum orders per agent per tick** | 10 |
| **Minimum order quantity** | 1 share |
| **Fractional shares** | Not allowed — quantity must be a positive integer |
| **Maximum order quantity** | No hard cap — subject to cash and concentration constraints |

Orders beyond the 10-per-tick limit are rejected with reason `tick_order_limit_exceeded`. The first 10 received (by server timestamp) are accepted; the rest are rejected immediately.

### 5.3 Short Selling

**Short selling is not allowed in Phase 1.**

- An agent cannot place a sell order for a symbol they do not currently hold.
- An agent cannot sell more shares than their current holding quantity.
- Violations are rejected with reason `insufficient_holdings`.

### 5.4 Order Submission Window

- Agents may only submit orders after receiving a `sim.tick` event for date D and before the Go service closes the submission window for date D.
- Orders submitted outside the window (e.g. during PAUSED status) are queued and processed on the next tick.

---

## 6. Portfolio Constraints

### 6.1 Single Stock Concentration Limit

| Parameter | Value |
|---|---|
| **Maximum allocation per stock** | 20% of total portfolio value at time of order |

**Calculation:**

```
portfolio_value         = cash_balance + Σ (holdings[s].quantity × prev_close[s])
max_allowed_in_stock    = 0.20 × portfolio_value
current_in_stock        = holdings[symbol].quantity × prev_close[symbol]
allowed_additional_cost = max_allowed_in_stock - current_in_stock
```

If a buy order's total cost (including brokerage) would push a single stock's allocation above 20%, the **entire order is rejected** with reason `concentration_limit_exceeded`. Partial fills are not supported in Phase 1.

### 6.2 Cash Constraint

An agent cannot spend more cash than they currently hold. If a market buy order's total cost (including brokerage) exceeds `cash_balance`, the order is rejected with reason `insufficient_cash`.

```
order_cost = (quantity × open_price) × (1 + brokerage_rate)
if order_cost > cash_balance → reject
```

### 6.3 No Leverage

Agents cannot borrow cash or trade on margin. The `cash_balance` is the only source of buying power at all times.

---

## 7. Order Execution & Pricing

### 7.1 Fill Price

| Order Type | Fill Price |
|---|---|
| Market Buy | `ohlcv.open` on tick date D |
| Market Sell | `ohlcv.open` on tick date D |

Using the open price ensures agents cannot benefit from knowing the full day's range — they submitted the order before the market "opened" that day.

### 7.2 Order Processing Logic

```
For each PENDING order on tick date D (processed in received-timestamp order):

  1. Fetch open_price = ohlcv[symbol][D].open
  2. Check order type is supported → else reject: order_type_not_supported
  3. Check symbol is in the Nifty 50 universe → else reject: invalid_symbol
  4. Check circuit breaker (Section 9.1) → if triggered reject: circuit_breaker_triggered
  5. Check liquidity limit (Section 9.2) → if exceeded reject: liquidity_limit_exceeded
  6. Check minimum price (Section 9.3) → if below ₹10 reject: below_minimum_price

  For BUY orders:
  7. Calculate order_cost = quantity × open_price × (1 + 0.001)
  8. Check cash_balance ≥ order_cost → else reject: insufficient_cash
  9. Check concentration limit → else reject: concentration_limit_exceeded
  10. Deduct order_cost from cash_balance
  11. Add quantity to holdings[symbol], update avg_buy_price
  12. Mark order: status=filled, filled_price=open_price, filled_at=D

  For SELL orders:
  7. Check holdings[symbol].quantity ≥ order.quantity → else reject: insufficient_holdings
  8. Calculate proceeds = quantity × open_price × (1 - 0.001)
  9. Deduct quantity from holdings[symbol]
  10. Add proceeds to cash_balance
  11. Mark order: status=filled, filled_price=open_price, filled_at=D
```

### 7.3 Slippage

No artificial slippage is applied in Phase 1. All market orders fill at the exact open price. Realistic slippage modelling will be considered in Phase 6.

### 7.4 Partial Fills

Not supported in Phase 1. An order is either fully filled or fully rejected. No partial quantities.

### 7.5 Portfolio Snapshot (EOD)

After all orders are processed for tick date D, the Go service calculates each agent's end-of-day portfolio value:

```
eod_total_value = cash_balance + Σ (holdings[s].quantity × ohlcv[s][D].close)
```

This value is saved to `portfolio_snapshots` and is used for leaderboard rankings and metric calculations.

---

## 8. Transaction Costs

All transaction costs are deducted from `cash_balance` at the time of order fill.

| Cost Component | Rate | Applied On |
|---|---|---|
| **Brokerage** | 0.1% of trade value | Both buy and sell |
| **STT (Securities Transaction Tax)** | 0.1% of trade value | Sell orders only |
| **Exchange transaction charge** | 0.00345% of trade value | Both buy and sell |
| **GST on brokerage** | 18% of brokerage amount | Both buy and sell |
| **SEBI turnover charge** | 0.0001% of trade value | Both buy and sell |
| **Stamp duty** | 0.015% of trade value | Buy orders only |

**Total effective cost (approximate):**

```
Buy  order: ~0.118% of trade value
Sell order: ~0.218% of trade value
```

**Example — Buy 10 shares of TCS.NS at ₹3,500 (trade value = ₹35,000):**

```
Brokerage (0.1%)              =   ₹35.00
GST on brokerage (18%)        =    ₹6.30
Exchange charge (0.00345%)    =    ₹1.21
SEBI charge (0.0001%)         =    ₹0.04
Stamp duty (0.015%)           =    ₹5.25
─────────────────────────────────────────
Total transaction cost        =   ₹47.80
Total deducted from cash      = ₹35,047.80
```

> **Implementation note:** The Go service computes all cost components individually and stores the breakdown in the `orders` table for auditability. The agent-facing API returns the net `filled_price` and `total_cost` in the order response.

---

## 9. Market Restrictions

### 9.1 Circuit Breaker (Stock Level)

If a stock's open price on day D has moved more than ±10% from the previous trading day's close, it is considered to have hit a circuit breaker. All orders for that symbol on tick date D are rejected.

```
price_change_pct = (D.open - prev_close) / prev_close × 100
if abs(price_change_pct) > 10:
    reject all orders for this symbol on date D
    reason: circuit_breaker_triggered
```

### 9.2 Liquidity Constraint

An agent's single order quantity cannot exceed 1% of the stock's total traded volume on the previous trading day. This prevents unrealistically large orders relative to actual market liquidity.

```
max_quantity = floor(0.01 × prev_day_volume[symbol])
if order.quantity > max_quantity:
    reject order
    reason: liquidity_limit_exceeded
```

### 9.3 Minimum Stock Price

Orders for stocks where `ohlcv.open < ₹10` on tick date D are rejected. This prevents agents from exploiting very low-priced stocks where small absolute movements produce outsized percentage gains.

```
if D.open < 10.00:
    reject order
    reason: below_minimum_price
```

---

## 10. Ranking & Win Condition

### 10.1 Primary Ranking Metric

Agents are ranked by **final total portfolio value** at the EOD snapshot of the last tick (31 December 2024).

```
final_value = cash_balance + Σ (holdings[symbol].quantity × last_day_close[symbol])
```

The agent with the highest `final_value` is the winner.

### 10.2 Secondary Metrics (Tiebreaker & Display)

Tiebreaker (if two agents have equal final value): **Sharpe Ratio** (higher wins).

All metrics below are computed at simulation end and displayed on the leaderboard:

| Metric | Definition |
|---|---|
| **Total Return %** | `(final_value − 10,00,000) / 10,00,000 × 100` |
| **Sharpe Ratio** | `mean(daily_returns) / std(daily_returns) × √252` — annualised, computed from EOD `portfolio_snapshots.total_value` |
| **Max Drawdown** | Largest peak-to-trough decline in EOD portfolio value over the simulation |
| **Win Rate** | `profitable_closed_trades / total_closed_trades × 100` |
| **Avg Holding Period** | Average number of trading days between a buy fill and the corresponding sell fill for the same symbol |
| **Turnover Rate** | `total_trade_value / avg_daily_portfolio_value` |
| **Sector Exposure** | % of portfolio value allocated per NSE sector (averaged over all EOD snapshots) |

### 10.3 Leaderboard Update Frequency

| Event | Leaderboard Action |
|---|---|
| After every tick (EOD snapshot) | Full recalculation persisted to DB, pushed to dashboard via WebSocket |
| On demand (API call) | Returns latest persisted leaderboard state |

---

## 11. Agent Behaviour Rules

### 11.1 Information Access

| Data | Accessible to Agent |
|---|---|
| OHLCV data for any Nifty 50 symbol up to and including `as_of_date` | ✅ Yes |
| Own portfolio — cash, holdings, unrealised P&L | ✅ Yes |
| Own order history — filled, rejected, pending | ✅ Yes |
| Leaderboard — other agents' total portfolio values | ✅ Yes |
| Other agents' holdings or individual trade history | ❌ No |
| OHLCV data for any date beyond `simulation.as_of_date` | ❌ No |
| Whether another agent placed an order on a specific symbol | ❌ No |

### 11.2 Decision Autonomy

- Each agent makes decisions independently using its own LLM model and prompting strategy.
- The Python orchestrator may use different prompt formats or context structures for different agents, but must supply the same underlying market data to all agents.
- Agents cannot communicate with each other during a simulation.
- Agents may maintain internal memory (e.g. a running list of past decisions) within their own service process between ticks.

### 11.3 Response Format

Agents must return a valid JSON array of orders. Any response that cannot be parsed as a valid JSON array is treated as a hold decision (`[]`) — no orders placed, no penalty applied.

```json
[
  { "symbol": "TCS.NS",      "side": "buy",  "order_type": "market", "quantity": 5  },
  { "symbol": "RELIANCE.NS", "side": "sell", "order_type": "market", "quantity": 10 }
]
```

An empty array `[]` is valid and means the agent chooses to take no action on that tick.

### 11.4 Rate Limiting

Each agent is limited to **100 API calls per simulation tick** to the Go service. Calls beyond this return HTTP 429. This prevents a runaway agent loop from flooding the simulator.

### 11.5 LLM Response Timeout

If an agent's LLM does not return a response within **30 seconds** of receiving the `sim.tick` event, the Python orchestrator treats that tick as a hold (`[]`) for that agent. The agent is not penalised — it simply takes no action that day.

---

## 12. Disqualification Conditions

An agent is **disqualified** (`status = disqualified`) and removed from active participation if any of the following conditions are met:

| Condition | Threshold |
|---|---|
| Portfolio value falls to zero or below | `total_value ≤ 0` |
| Agent service unreachable for consecutive ticks | 10 consecutive ticks with no response |
| Repeated API abuse causing server-side errors | 500 consecutive HTTP 4xx errors |

A disqualified agent's portfolio is frozen at the value recorded at the time of disqualification. They continue to appear on the leaderboard with a `[DQ]` marker. Their historical `portfolio_snapshots` are retained for post-simulation analysis.

---

## 13. Versioning & Change Log

| Version | Date | Author | Changes |
|---|---|---|---|
| `v1.0` | May 2026 | — | Initial rules document |

> **Critical:** Any change to Sections 5–10 (order rules, portfolio constraints, execution, costs, market restrictions, ranking) requires a version number increment and must be deployed to both the Go service and Python service before the next simulation run. Rules must **never** change during an active simulation.

---

*This document is the source of truth for all simulation behaviour. When in doubt, implement exactly what is written here.*