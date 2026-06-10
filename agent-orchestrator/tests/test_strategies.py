import pytest

from agents.factory import create_agent
from agents.strategies import IndexHuggerAgent, MeanReversionAgent
from market_client.models import Order


@pytest.mark.asyncio
async def test_mean_reversion_buys_oversold():
    agent = MeanReversionAgent("a1", "s1", {"go_service_url": "http://x", "api_key": "k"})
    closes = [100.0] * 9 + [90.0]
    agent._last_portfolio = {"cash": 200_000, "holdings": []}
    agent._last_market_data = {"TCS.NS": [{"close": c} for c in closes]}
    orders = await agent.decide("ctx")
    assert len(orders) == 1
    assert orders[0].side == "buy"
    assert orders[0].symbol == "TCS.NS"


@pytest.mark.asyncio
async def test_mean_reversion_sells_on_recovery():
    agent = MeanReversionAgent("a1", "s1", {"go_service_url": "http://x", "api_key": "k"})
    closes = [100.0] * 9 + [103.0]
    agent._last_portfolio = {
        "cash": 50_000,
        "holdings": [{"symbol": "TCS.NS", "quantity": 10}],
    }
    agent._last_market_data = {"TCS.NS": [{"close": c} for c in closes]}
    orders = await agent.decide("ctx")
    assert len(orders) == 1
    assert orders[0].side == "sell"


@pytest.mark.asyncio
async def test_index_hugger_buys_underweight():
    agent = IndexHuggerAgent("a1", "s1", {"go_service_url": "http://x", "api_key": "k"})
    agent.context_symbol_count = 2
    watch = agent._watchlist()
    agent._last_portfolio = {
        "cash": 500_000,
        "total_value": 1_000_000,
        "holdings": [],
    }
    agent._last_market_data = {sym: [{"close": 100 + i * 10}] for i, sym in enumerate(watch)}
    orders = await agent.decide("ctx")
    assert orders
    assert all(isinstance(o, Order) for o in orders)
    assert all(o.side == "buy" for o in orders)


def test_factory_creates_mean_reversion_strategy():
    entry = {
        "name": "mr",
        "provider": "custom",
        "model": "mean-reversion-v1",
        "agent_id": "00000000-0000-4000-8000-000000000003",
        "api_key": "secret",
    }
    agent = create_agent(entry["agent_id"], "sim-id", entry, "http://localhost:8070")
    assert agent.__class__.__name__ == "MeanReversionAgent"
