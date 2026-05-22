from __future__ import annotations

from agents.base_agent import BaseAgent
from market_client.models import Order


class CustomAgent(BaseAgent):
    """Simple rule-based agent: buy on short-term momentum, otherwise hold."""

    momentum_threshold: float = 1.02
    cash_fraction: float = 0.05

    async def decide(self, context: str) -> list[Order]:
        portfolio = self._last_portfolio
        market_data = self._last_market_data
        cash = float(portfolio.get("cash", 0))
        if cash < 10_000:
            return []

        for symbol, bars in market_data.items():
            if len(bars) < 2:
                continue
            first_close = float(bars[0].get("close", 0))
            last_close = float(bars[-1].get("close", 0))
            if first_close <= 0 or last_close <= first_close * self.momentum_threshold:
                continue
            spend = cash * self.cash_fraction
            quantity = max(1, int(spend / last_close))
            return [Order(symbol=symbol, side="buy", quantity=quantity)]

        return []
