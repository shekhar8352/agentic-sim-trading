from __future__ import annotations

import logging
from abc import ABC, abstractmethod
from typing import Any

from agents.universe import NIFTY_50
from market_client.client import MarketClient
from market_client.errors import APIError, MarketClientError
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
        self._last_hourly_data: dict[str, list[dict]] = {}

    def _watchlist(self) -> tuple[str, ...]:
        count = int(self.config.get("context_symbol_count", self.context_symbol_count))
        count = max(1, min(count, len(NIFTY_50)))
        return NIFTY_50[:count]

    async def build_context(self, current_date: str, tick: dict[str, Any] | None = None) -> str:
        tick = tick or {}
        try:
            portfolio = await self.client.get_portfolio()
            market_data: dict[str, list[dict]] = {}
            hourly_data: dict[str, list[dict]] = {}
            for symbol in self._watchlist():
                market_data[symbol] = await self.client.get_ohlcv(symbol, days=20)
            if str(tick.get("interval") or "") == "60m":
                for symbol in self._watchlist():
                    hourly_data[symbol] = await self.client.get_ohlcv(
                        symbol, days=20, interval="60m", bars=20
                    )

            self._last_portfolio = portfolio
            self._last_market_data = market_data
            self._last_hourly_data = hourly_data
        except MarketClientError as exc:
            if self._last_portfolio:
                logger.warning(
                    "agent=%s market fetch failed (%s), using cached context",
                    self.agent_id,
                    exc,
                )
                portfolio = self._last_portfolio
                market_data = self._last_market_data
                hourly_data = getattr(self, "_last_hourly_data", {})
            else:
                raise
        return build_trading_context(
            current_date, portfolio, market_data, tick=tick, hourly_data=hourly_data
        )

    @abstractmethod
    async def decide(self, context: str) -> list[Order]:
        """Return orders to place this turn (empty list = hold)."""

    async def run_turn(
        self, current_date: str, tick: dict[str, Any] | None = None
    ) -> list[dict[str, Any]]:
        tick = tick or {}
        cadence = int(self.config.get("decision_every_n_bars", 1) or 1)
        if cadence < 1:
            cadence = 1
        idx = tick.get("trading_day_index")
        if idx is not None and cadence > 1 and int(idx) % cadence != 0:
            logger.info(
                "agent=%s skip decide cadence=%s index=%s",
                self.agent_id,
                cadence,
                idx,
            )
            return []

        try:
            context = await self.build_context(current_date, tick=tick)
        except MarketClientError as exc:
            logger.warning(
                "agent=%s skip turn: cannot build context (%s)",
                self.agent_id,
                exc,
            )
            return []

        try:
            orders = await self.decide(context)
        except Exception as exc:
            logger.warning(
                "agent=%s skip turn: decide failed (%s)",
                self.agent_id,
                exc,
            )
            return []

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
                    "agent=%s order=%s rejected error=%s",
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
            except MarketClientError as exc:
                logger.warning(
                    "agent=%s order=%s transport error=%s",
                    self.agent_id,
                    order.model_dump(),
                    exc,
                )
                results.append(
                    {
                        "order": order.model_dump(),
                        "error": str(exc),
                    }
                )
        return results

    async def act(self, current_date: str) -> dict[str, Any]:
        """Compatibility wrapper used by orchestrator until Step 14 wiring."""
        placed = await self.run_turn(current_date)
        return {"orders": placed}
