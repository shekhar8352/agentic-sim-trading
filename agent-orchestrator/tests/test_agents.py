from __future__ import annotations

from unittest.mock import AsyncMock

import httpx
import pytest

from agents.base_agent import BaseAgent
from agents.custom_agent import CustomAgent
from agents.llm_agent import LLMAgent
from market_client.errors import APIError, TransportError
from market_client.models import Order
from orchestrator.resilience import compute_backoff_delay


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
async def test_build_context_includes_hourly_when_tick_is_60m():
    agent = StubAgent("agent-1", "sim-1", {"go_service_url": "http://x", "api_key": "k"})
    agent.client.get_portfolio = AsyncMock(
        return_value={"cash": 1000, "holdings": [], "total_value": 1000, "total_return_pct": 0}
    )
    agent.client.get_ohlcv = AsyncMock(
        return_value=[{"date": "2025-03-12", "ts": "2025-03-12T03:45:00Z", "open": 1, "high": 1, "low": 1, "close": 100, "volume": 1}]
    )
    text = await agent.build_context(
        "2025-03-12",
        tick={"interval": "60m", "bar_ts": "2025-03-12T03:45:00Z", "session_bar": 2, "session_bars": 7},
    )
    assert "bar 2/7" in text
    assert "HOURLY" in text
    assert agent.client.get_ohlcv.await_count == 20


@pytest.mark.asyncio
async def test_run_turn_skips_decide_on_cadence():
    agent = StubAgent(
        "agent-1",
        "sim-1",
        {"go_service_url": "http://x", "api_key": "k", "decision_every_n_bars": 3},
    )
    agent.build_context = AsyncMock(return_value="ctx")
    agent.client.place_order = AsyncMock(return_value={"status": "pending"})
    results = await agent.run_turn("2025-03-12", tick={"trading_day_index": 1})
    assert results == []
    agent.build_context.assert_not_awaited()
    results = await agent.run_turn("2025-03-12", tick={"trading_day_index": 3})
    assert len(results) == 1


@pytest.mark.asyncio
async def test_build_context_uses_cache_when_market_unavailable():
    agent = StubAgent("agent-1", "sim-1", {"go_service_url": "http://x", "api_key": "k"})
    agent._last_portfolio = {
        "cash": 500,
        "holdings": [],
        "total_value": 500,
        "total_return_pct": 0,
    }
    agent._last_market_data = {"RELIANCE.NS": [{"close": 99}]}
    agent.client.get_portfolio = AsyncMock(
        side_effect=TransportError("GET /api/v1/portfolio timed out")
    )

    text = await agent.build_context("2024-01-02")
    assert "₹500.00" in text
    assert "RELIANCE.NS" in text


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
async def test_run_turn_skips_when_context_unavailable():
    agent = StubAgent("agent-1", "sim-1", {"go_service_url": "http://x", "api_key": "k"})
    agent.build_context = AsyncMock(
        side_effect=TransportError("GET /api/v1/portfolio transport error")
    )

    results = await agent.run_turn("2024-01-02")
    assert results == []


@pytest.mark.asyncio
async def test_run_turn_logs_go_order_rejection():
    agent = StubAgent("agent-1", "sim-1", {"go_service_url": "http://x", "api_key": "k"})
    agent.build_context = AsyncMock(return_value="ctx")
    agent.client.place_order = AsyncMock(
        side_effect=APIError(400, "unknown symbol: FAKE.NS")
    )

    results = await agent.run_turn("2024-01-02")
    assert len(results) == 1
    assert results[0]["error"] == "unknown symbol: FAKE.NS"
    assert results[0]["status_code"] == 400


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
    def __init__(self, response: str = "", *, fail_times: int = 0):
        super().__init__("a", "s", {"go_service_url": "http://x", "api_key": "k"})
        self._response = response
        self._fail_times = fail_times
        self._calls = 0

    async def complete(self, context: str) -> str:
        self._calls += 1
        if self._calls <= self._fail_times:
            raise httpx.ReadTimeout("read timed out")
        return self._response


@pytest.mark.asyncio
async def test_llm_agent_parses_decision():
    agent = StubLLM('[{"symbol":"TCS.NS","side":"buy","order_type":"market","quantity":2}]')
    orders = await agent.decide("ctx")
    assert len(orders) == 1
    assert orders[0].quantity == 2


@pytest.mark.asyncio
async def test_llm_agent_hold_array_is_valid():
    agent = StubLLM("[]")
    orders = await agent.decide("ctx")
    assert orders == []


@pytest.mark.asyncio
async def test_llm_agent_malformed_json_returns_empty():
    agent = StubLLM("not json at all")
    orders = await agent.decide("ctx")
    assert orders == []


@pytest.mark.asyncio
async def test_llm_agent_retries_once_on_timeout():
    agent = StubLLM(
        '[{"symbol":"TCS.NS","side":"buy","order_type":"market","quantity":1}]',
        fail_times=1,
    )
    orders = await agent.decide("ctx")
    assert len(orders) == 1
    assert agent._calls == 2


@pytest.mark.asyncio
async def test_llm_agent_skips_turn_after_retry_exhausted():
    agent = StubLLM(fail_times=2)
    orders = await agent.decide("ctx")
    assert orders == []
    assert agent._calls == 2


def test_compute_backoff_delay_caps_at_max():
    assert compute_backoff_delay(1) == 1.0
    assert compute_backoff_delay(2) == 2.0
    assert compute_backoff_delay(10, max_delay_seconds=60.0) == 60.0
