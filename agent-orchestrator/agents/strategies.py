"""Rule-based trading strategies (roadmap Step 19) for custom / no-LLM agents."""

from __future__ import annotations

from agents.base_agent import BaseAgent
from agents.custom_agent import CustomAgent
from market_client.models import Order


def _closes(bars: list[dict]) -> list[float]:
    return [float(b.get("close", 0)) for b in bars if float(b.get("close", 0)) > 0]


def _avg(values: list[float]) -> float:
    return sum(values) / len(values) if values else 0.0


class MeanReversionAgent(BaseAgent):
    """Buy oversold names (below 10-day mean); sell on recovery."""

    oversold_ratio: float = 0.95
    recovery_ratio: float = 1.02
    cash_fraction: float = 0.08
    min_cash: float = 10_000.0

    async def decide(self, context: str) -> list[Order]:
        portfolio = self._last_portfolio
        market_data = self._last_market_data
        cash = float(portfolio.get("cash", 0))
        holdings = {h.get("symbol"): int(h.get("quantity", 0)) for h in portfolio.get("holdings", [])}

        orders: list[Order] = []

        for symbol, qty in holdings.items():
            if qty <= 0:
                continue
            bars = market_data.get(symbol, [])
            closes = _closes(bars)
            if len(closes) < 5:
                continue
            window = closes[-10:]
            mean = _avg(window)
            last = closes[-1]
            if mean > 0 and last >= mean * self.recovery_ratio:
                orders.append(Order(symbol=symbol, side="sell", quantity=qty))
                if len(orders) >= self.max_orders_per_turn:
                    return orders

        if cash < self.min_cash:
            return orders

        for symbol, bars in market_data.items():
            if holdings.get(symbol, 0) > 0:
                continue
            closes = _closes(bars)
            if len(closes) < 10:
                continue
            window = closes[-10:]
            mean = _avg(window)
            last = closes[-1]
            if mean <= 0 or last > mean * self.oversold_ratio:
                continue
            spend = cash * self.cash_fraction
            quantity = max(1, int(spend / last))
            orders.append(Order(symbol=symbol, side="buy", quantity=quantity))
            break

        return orders


class IndexHuggerAgent(BaseAgent):
    """Baseline strategy: equal-weight top watchlist names (Nifty-like basket)."""

    target_positions: int = 8
    rebalance_fraction: float = 0.12
    min_cash: float = 10_000.0

    async def decide(self, context: str) -> list[Order]:
        portfolio = self._last_portfolio
        market_data = self._last_market_data
        cash = float(portfolio.get("cash", 0))
        total_value = float(portfolio.get("total_value", cash))
        if total_value <= 0:
            return []

        holdings = {
            h.get("symbol"): int(h.get("quantity", 0))
            for h in portfolio.get("holdings", [])
            if h.get("symbol")
        }
        symbols = [s for s in self._watchlist() if s in market_data][: self.target_positions]
        if not symbols:
            return []

        target_each = total_value / len(symbols)
        orders: list[Order] = []

        for symbol in symbols:
            bars = market_data[symbol]
            closes = _closes(bars)
            if not closes:
                continue
            last = closes[-1]
            if last <= 0:
                continue
            current_qty = holdings.get(symbol, 0)
            current_value = current_qty * last
            gap = target_each - current_value

            if gap > target_each * 0.05 and cash >= self.min_cash:
                spend = min(cash * self.rebalance_fraction, gap)
                qty = max(1, int(spend / last))
                orders.append(Order(symbol=symbol, side="buy", quantity=qty))
                cash -= qty * last
            elif gap < -target_each * 0.08 and current_qty > 0:
                qty = max(1, int(min(current_qty, (-gap) / last)))
                orders.append(Order(symbol=symbol, side="sell", quantity=qty))

            if len(orders) >= self.max_orders_per_turn:
                break

        return orders


CUSTOM_STRATEGY_MODELS: dict[str, type[BaseAgent]] = {
    "momentum-v1": CustomAgent,
    "mean-reversion-v1": MeanReversionAgent,
    "index-hugger-v1": IndexHuggerAgent,
}

STRATEGY_CATALOG: tuple[dict[str, str], ...] = (
    {
        "id": "momentum-v1",
        "label": "Momentum",
        "description": "Buys stocks with short-term upward price momentum.",
    },
    {
        "id": "mean-reversion-v1",
        "label": "Mean Reversion",
        "description": "Buys oversold dips below the 10-day average; sells on recovery.",
    },
    {
        "id": "index-hugger-v1",
        "label": "Index Hugger",
        "description": "Equal-weight basket across top Nifty names — baseline control strategy.",
    },
)
