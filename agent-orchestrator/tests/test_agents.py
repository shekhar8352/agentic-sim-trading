from __future__ import annotations

from unittest.mock import AsyncMock

import pytest

from agents.base_agent import BaseAgent
from agents.custom_agent import CustomAgent
from agents.llm_agent import LLMAgent
from market_client.models import Order


class StubAgent(BaseAgent):
    async def decide(self, context: str) -> list[Order]:
        return [Order(symbol="TCS.NS", side="buy", quantity=1)]


@pytest.mark.asyncio
async def test_build_context_fetches_portfolio_and_ohlcv():
    agent = StubAgent("agent-1", "sim-1", {"go_service_url": "http://x", "api_key": "k"})
    agent.client.get_portfolio = AsyncMock(
        return_value={"cash": 1000, "holdings": [], "total_value": 1000, "total_return_pct": 0}
    )
    agent.client.get_ohlcv = AsyncMock(return_value=[{"close": 100}])

    text = await agent.build_context("2024-01-02")
    assert "2024-01-02" in text
    assert "₹1,000.00" in text
    agent.client.get_portfolio.assert_awaited_once()
    assert agent.client.get_ohlcv.await_count == 10


@pytest.mark.asyncio
async def test_run_turn_places_orders():
    agent = StubAgent("agent-1", "sim-1", {"go_service_url": "http://x", "api_key": "k"})
    agent.build_context = AsyncMock(return_value="ctx")
    agent.client.place_order = AsyncMock(return_value={"status": "pending"})

    results = await agent.run_turn("2024-01-02")
    assert len(results) == 1
    assert results[0]["result"]["status"] == "pending"
    agent.client.place_order.assert_awaited_once()


@pytest.mark.asyncio
async def test_custom_agent_momentum_buy():
    agent = CustomAgent("agent-1", "sim-1", {"go_service_url": "http://x", "api_key": "k"})
    agent._last_portfolio = {"cash": 100_000}
    agent._last_market_data = {
        "RELIANCE.NS": [{"close": 100}, {"close": 110}],
    }
    orders = await agent.decide("ignored")
    assert len(orders) == 1
    assert orders[0].symbol == "RELIANCE.NS"
    assert orders[0].side == "buy"


class StubLLM(LLMAgent):
    def __init__(self, response: str):
        super().__init__("a", "s", {"go_service_url": "http://x", "api_key": "k"})
        self._response = response

    async def complete(self, context: str) -> str:
        return self._response


@pytest.mark.asyncio
async def test_llm_agent_parses_decision():
    agent = StubLLM('[{"symbol":"TCS.NS","side":"buy","order_type":"market","quantity":2}]')
    orders = await agent.decide("ctx")
    assert len(orders) == 1
    assert orders[0].quantity == 2
