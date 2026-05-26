from __future__ import annotations

import httpx
import pytest

from market_client.client import MarketClient
from market_client.errors import APIError, ConfigurationError, TransportError
from market_client.models import Order


def _client(transport: httpx.AsyncBaseTransport) -> MarketClient:
    return MarketClient(
        "http://sim.test",
        agent_id="00000000-0000-4000-8000-000000000001",
        api_key="secret-key",
        simulation_id="00000000-0000-4000-8000-000000000099",
        http_client=httpx.AsyncClient(transport=transport, base_url="http://sim.test"),
    )


@pytest.mark.asyncio
async def test_get_portfolio_success():
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.headers["X-Agent-ID"] == "00000000-0000-4000-8000-000000000001"
        assert request.headers["X-API-Key"] == "secret-key"
        assert request.url.params["simulation_id"] == "00000000-0000-4000-8000-000000000099"
        return httpx.Response(200, json={"cash": 1_000_000, "holdings": []})

    client = _client(httpx.MockTransport(handler))
    data = await client.get_portfolio()
    assert data["cash"] == 1_000_000
    await client.aclose()


@pytest.mark.asyncio
async def test_get_ohlcv_and_quote():
    calls: list[str] = []

    def handler(request: httpx.Request) -> httpx.Response:
        calls.append(request.url.path)
        if "/ohlcv/" in request.url.path:
            return httpx.Response(
                200,
                json=[{"symbol": "RELIANCE.NS", "date": "2024-01-02", "close": 2500.0}],
            )
        return httpx.Response(200, json={"symbol": "RELIANCE.NS", "close": 2500.0})

    client = _client(httpx.MockTransport(handler))
    bars = await client.get_ohlcv("RELIANCE.NS", days=20)
    quote = await client.get_quote("RELIANCE.NS")
    assert len(bars) == 1
    assert quote["close"] == 2500.0
    assert any("/market/ohlcv/" in p for p in calls)
    assert any("/market/quote/" in p for p in calls)
    await client.aclose()


@pytest.mark.asyncio
async def test_place_order_payload():
    captured: dict = {}

    def handler(request: httpx.Request) -> httpx.Response:
        import json

        captured["body"] = json.loads(request.content.decode())
        return httpx.Response(201, json={"id": "order-1", "status": "pending"})

    client = _client(httpx.MockTransport(handler))
    order = Order(symbol="TCS.NS", side="buy", quantity=5)
    result = await client.place_order(order)
    assert result["status"] == "pending"
    assert captured["body"]["agent_id"] == client.agent_id
    assert captured["body"]["symbol"] == "TCS.NS"
    assert captured["body"]["simulation_id"] == client.simulation_id
    await client.aclose()


@pytest.mark.asyncio
async def test_list_and_cancel_orders():
    def handler(request: httpx.Request) -> httpx.Response:
        if request.method == "GET":
            return httpx.Response(200, json=[{"id": "o1", "status": "pending"}])
        if request.method == "DELETE":
            assert request.url.params["simulation_id"]
            return httpx.Response(200, json={"detail": "canceled"})
        return httpx.Response(405)

    client = _client(httpx.MockTransport(handler))
    orders = await client.list_orders(limit=50)
    assert orders[0]["id"] == "o1"
    resp = await client.cancel_order("o1")
    assert resp["detail"] == "canceled"
    await client.aclose()


@pytest.mark.asyncio
async def test_api_error_includes_detail():
    def handler(_: httpx.Request) -> httpx.Response:
        return httpx.Response(401, json={"detail": "invalid credentials"})

    client = _client(httpx.MockTransport(handler))
    with pytest.raises(APIError) as exc:
        await client.get_portfolio()
    assert exc.value.status_code == 401
    assert exc.value.detail == "invalid credentials"
    await client.aclose()


@pytest.mark.asyncio
async def test_transport_error_on_connect_failure():
    def handler(_: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("connection refused")

    client = _client(httpx.MockTransport(handler))
    with pytest.raises(TransportError) as exc:
        await client.get_portfolio()
    assert "transport error" in str(exc.value).lower()
    await client.aclose()


@pytest.mark.asyncio
async def test_transport_error_on_timeout():
    def handler(_: httpx.Request) -> httpx.Response:
        raise httpx.ReadTimeout("read timed out")

    client = _client(httpx.MockTransport(handler))
    with pytest.raises(TransportError) as exc:
        await client.get_portfolio()
    assert "timed out" in str(exc.value).lower()
    await client.aclose()


@pytest.mark.asyncio
async def test_missing_simulation_id_raises():
    client = MarketClient("http://sim.test", "agent", "key")
    with pytest.raises(ConfigurationError):
        await client.get_portfolio()
    await client.aclose()


def test_order_side_validation():
    with pytest.raises(ValueError):
        Order(symbol="TCS.NS", side="hold", quantity=1)  # type: ignore[arg-type]
