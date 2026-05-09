# Simulation Rules

This document is the **authoritative specification** for how the AI agent trading simulation behaves. It corresponds to **Phase 0 — Foundation & Planning**: parameters must be locked before implementing the market simulator and orchestrator. Changes here should be treated as a **versioned contract**; active runs store a snapshot of applicable rules in `simulations.config` (see roadmap schema).

---

## 1. Scope & assumptions

- **Markets:** Indian equities (NSE primary; symbols use `yfinance` style, e.g. `RELIANCE.NS`).
- **Data:** Historical only — no live quotes. **End-of-day (EOD)** bars drive each decision.
- **Time model:** **One simulation tick = one trading calendar day** in the backtest window. Only dates with OHLCV data for the traded symbol advance the clock for that decision (align with unified session calendar in implementation).
- **Currency:** Indian Rupee (**₹**). All cash, notionals, and metrics are in ₹ unless stated otherwise.

---

## 2. Simulation parameters (baseline)

| Parameter | Value | Notes |
|-----------|--------|--------|
| Starting capital per agent | ₹10,00,000 | Same for every agent unless a run overrides in config. |
| Stock universe | Nifty 50 | Start small; extend only by explicit config change. |
| Simulation time unit | 1 tick = 1 trading day | EOD decision after that day’s bar is known (no lookahead within the same bar). |
| Historical data range | 2022-01-01 to 2024-12-31 | Inclusive of valid trading days; ingest/source must match. |
| Order types (Phase 1) | **Market orders only** | Limit/stop/other types are out of scope until explicitly added to this doc. |
| Short selling (Phase 1) | **Disallowed** | Agents may only sell shares they hold. |
| Max capital per single stock | **20%** of **current total portfolio value** (cash + mark-to-market holdings) | Measured at **order validation** time using EOD prices for the tick. |
| Max orders per agent per tick | **10** | Combined buy + sell submissions in one tick. |
| Brokerage | **0.1%** of trade value per **side** | Applied on executed notional (buy and sell each pay 0.1% unless a future revision says otherwise). |
| Slippage / spread | **None** in baseline | Market fills at a defined reference price (see §4); add slippage only when documented here. |

---

## 3. Agent behavior & fairness

- Each agent receives the **same** public information available at that tick: prices/volume through the current date, portfolio state, universe definition, and **these rules**.
- **No lookahead:** Decisions for date *D* may use OHLCV and derived features for dates ≤ *D* only. The simulator must not expose future bars.
- **Identity:** Each agent is identified in the system (`agents` table); one portfolio per agent per simulation.
- **API keys & models:** Stored per agent for orchestration; they must not confer access pe-time to future data.

---

## 4. Pricing & valuation

- **Mark-to-market:** Holdings are valued at the **official close** of symbol *S* on tick date *D* from the `ohlcv` table (`close`).
- **Market order fill price (baseline):** **Close** of *D* for *S* (after a buy/sell is accepted). Document any later change to open/VWAP here.
- **Total portfolio value:** Cash + Σ (quantity × close on *D*) for all holdings.
- **Partial shares:** **Not allowed** — order quantities are whole units (integer shares).

---

## 5. Orders & lifecycle

- **Sides:** `buy` | `sell`.
- **States:** `pending` → `filled` | `rejected` | `cancelled` (cancellation only if/when supported and documented).
- **Lifecycle per tick:** Agents submit orders during the tick; the Go simulator validates, matches at EOD reference price, updates cash/holdings, applies brokerage, persists `orders` and portfolio state.
- **Sell orders:** Quantity must not exceed current holding for that symbol.
- **Buy orders:** Must not violate §2 max capital per stock or available cash after fees.

### 5.1 Rejection reasons (non-exhaustive)

The engine should set `rejection_reason` (or equivalent) when an order does not execute, including:

- `INSUFFICIENT_CASH` — not enough cash for buy + brokerage.
- `INSUFFICIENT_POSITION` — sell larger than holdings.
- `CONCENTRATION_LIMIT` — would exceed max % in one stock.
- `ORDER_RATE_LIMIT` — more than max orders this tick.
- `INVALID_SYMBOL` — not in universe or no data for the date.
- `INVALID_SIDE_OR_QTY` — malformed or non-positive quantity.
- `SIMULATION_STATE` — simulation not running or tick not advanced.

---

## 6. Cash & brokerage accounting

- On **buy fill:** Debit `quantity × fill_price`; debit **brokerage** = `0.1% × (quantity × fill_price)`.
- On **sell fill:** Credit `quantity × fill_price`; debit **brokerage** = `0.1% × (quantity × fill_price)`.
- **Cash must never go negative** after a fill and fee deduction.

---

## 7. Ranking & win condition

- **Primary:** Final **total portfolio value** at end of simulation (after last tick’s marks and settled trades).
- **Secondary (tie-break / risk-adjusted view):** **Sharpe ratio** of daily portfolio returns over the simulation path (use a documented risk-free rate, e.g. **0** for baseline, or a fixed annual rate stored in config).

Leaderboard ordering:

1. Higher final total value wins.
2. If tied, higher Sharpe wins.
3. Further ties: optional deterministic tie-break (e.g. lower cumulative transaction cost, then agent id).

---

## 8. Data & universe rules

- Only symbols in the **active universe** for that simulation are tradable.
- If a symbol has **missing** OHLCV on a date, the engine must define behavior (e.g. **no new entries** for that symbol that day, pending orders cancelled or rejected — choose one policy per implementation and record it in release notes until codified here).

---

## 9. Communication & infrastructure (reference)

Aligned with Phase 0 / architecture:

- **Go service:** Source of truth for clock, matching, portfolios, REST API for placements/quotes/state.
- **Python orchestrator:** Agents, prompts, LLM calls; consumes Go API and Redis events as per `docs/rroadmap.md`.
- **PostgreSQL:** Shared historical and portfolio persistence; **Redis:** pub/sub for ticks/fills as designed.

---

## 10. Rule changes & versioning

- Any change to §§2–8 must be **reflected in this file** and, for active or reproducible runs, copied into `simulations.config` JSON so historical results remain explainable.
- **Phase gates:** Features such as short selling, limit orders, intraday ticks, or alternate fill models are **invalid** until this document is updated and the version noted below.

---

## Document version

| Version | Date | Summary |
|---------|------|---------|
| 0.1.0 | 2026-05-10 | Initial baseline from Phase 0 roadmap (Nifty 50, EOD, market-only, no shorts). |
