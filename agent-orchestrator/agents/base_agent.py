from __future__ import annotations

import logging
from abc import ABC, abstractmethod
from typing import Any

from agents.universe import NIFTY_50
from market_client.client import MarketClient
from market_client.errors import APIError
from market_client.models import Order
from prompts.templates import build_trading_context

logger = logging.getLogger(__name__)


class BaseAgent(ABC):
    """Trading agent base: context building, decision, and order submission."""

    context_symbol_count: int = 10
    max_orders_per_turn: int = 10

    def __init__(self, agent_id: str, sim_id: str, config: dict[str, Any]):
        self.agent_id = agent_id
        self.sim_id = sim_id
        self.config = config
        self.client = MarketClient(
            config["go_service_url"],
            agent_id,
            config["api_key"],
            simulation_id=sim_id,
        )
        self._last_portfolio: dict[str, Any] = {}
        self._last_market_data: dict[str, list[dict]] = {}

    def _watchlist(self) -> tuple[str, ...]:
        count = int(self.config.get("context_symbol_count", self.context_symbol_count))
        count = max(1, min(count, len(NIFTY_50)))
        return NIFTY_50[:count]

    async def build_context(self, current_date: str) -> str:
        portfolio = await self.client.get_portfolio()
        market_data: dict[str, list[dict]] = {}
        for symbol in self._watchlist():
            market_data[symbol] = await self.client.get_ohlcv(symbol, days=20)

        self._last_portfolio = portfolio
        self._last_market_data = market_data
        return build_trading_context(current_date, portfolio, market_data)

    @abstractmethod
    async def decide(self, context: str) -> list[Order]:
        """Return orders to place this turn (empty list = hold)."""

    async def run_turn(self, current_date: str) -> list[dict[str, Any]]:
        context = await self.build_context(current_date)
        orders = await self.decide(context)
        results: list[dict[str, Any]] = []

        for order in orders[: self.max_orders_per_turn]:
            try:
                result = await self.client.place_order(order)
                logger.info(
                    "agent=%s order=%s status=%s",
                    self.agent_id,
                    order.model_dump(),
                    result.get("status"),
                )
                results.append({"order": order.model_dump(), "result": result})
            except APIError as exc:
                logger.warning(
                    "agent=%s order=%s error=%s",
                    self.agent_id,
                    order.model_dump(),
                    exc.detail,
                )
                results.append(
                    {
                        "order": order.model_dump(),
                        "error": exc.detail,
                        "status_code": exc.status_code,
                    }
                )
        return results

    async def act(self, current_date: str) -> dict[str, Any]:
        """Compatibility wrapper used by orchestrator until Step 14 wiring."""
        placed = await self.run_turn(current_date)
        return {"orders": placed}
