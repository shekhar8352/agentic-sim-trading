"""HTTP client for the Go market-simulator API (roadmap Step 12)."""

from __future__ import annotations

from types import TracebackType
from typing import Any

import httpx

from market_client.errors import APIError, ConfigurationError
from market_client.models import Order


def _parse_error_detail(response: httpx.Response) -> str:
    try:
        body = response.json()
        if isinstance(body, dict) and body.get("detail"):
            return str(body["detail"])
    except Exception:
        pass
    text = response.text.strip()
    return text or response.reason_phrase or "request failed"


class MarketClient:
    """Async client for agent-authenticated market-simulator routes."""

    def __init__(
        self,
        base_url: str,
        agent_id: str,
        api_key: str,
        simulation_id: str | None = None,
        *,
        timeout: float = 30.0,
        http_client: httpx.AsyncClient | None = None,
    ):
        self.base_url = base_url.rstrip("/")
        self.agent_id = agent_id
        self.simulation_id = simulation_id
        self.headers = {"X-Agent-ID": agent_id, "X-API-Key": api_key}
        self._timeout = timeout
        self._http = http_client
        self._owns_client = http_client is None

    async def __aenter__(self) -> MarketClient:
        await self._ensure_client()
        return self

    async def __aexit__(
        self,
        exc_type: type[BaseException] | None,
        exc: BaseException | None,
        tb: TracebackType | None,
    ) -> None:
        await self.aclose()

    async def _ensure_client(self) -> httpx.AsyncClient:
        if self._http is None:
            self._http = httpx.AsyncClient(
                base_url=self.base_url,
                timeout=self._timeout,
                headers=self.headers,
            )
            self._owns_client = True
        return self._http

    async def aclose(self) -> None:
        if self._http is not None and self._owns_client:
            await self._http.aclose()
        self._http = None

    def _resolve_simulation_id(self, simulation_id: str | None) -> str:
        sim = simulation_id or self.simulation_id
        if not sim:
            raise ConfigurationError(
                "simulation_id is required (pass per call or set on MarketClient)"
            )
        return sim

    async def _request(
        self,
        method: str,
        path: str,
        *,
        params: dict[str, Any] | None = None,
        json: dict[str, Any] | None = None,
        auth: bool = True,
    ) -> Any:
        client = await self._ensure_client()
        headers = self.headers if auth else None
        response = await client.request(method, path, params=params, json=json, headers=headers)
        if response.is_error:
            raise APIError(response.status_code, _parse_error_detail(response))
        if response.status_code == 204 or not response.content:
            return {}
        return response.json()

    async def health(self) -> dict[str, Any]:
        """GET /health (no agent auth)."""
        return await self._request("GET", "/health", auth=False)

    async def get_portfolio(self, simulation_id: str | None = None) -> dict[str, Any]:
        """GET /api/v1/portfolio/{agent_id}?simulation_id=..."""
        sim = self._resolve_simulation_id(simulation_id)
        return await self._request(
            "GET",
            f"/api/v1/portfolio/{self.agent_id}",
            params={"simulation_id": sim},
        )

    async def get_quote(self, symbol: str, simulation_id: str | None = None) -> dict[str, Any]:
        """GET /api/v1/market/quote/{symbol}?simulation_id=..."""
        sim = self._resolve_simulation_id(simulation_id)
        symbol = symbol.strip()
        return await self._request(
            "GET",
            f"/api/v1/market/quote/{symbol}",
            params={"simulation_id": sim},
        )

    async def get_ohlcv(
        self,
        symbol: str,
        days: int = 30,
        simulation_id: str | None = None,
    ) -> list[dict[str, Any]]:
        """GET /api/v1/market/ohlcv/{symbol}?days=&simulation_id=..."""
        sim = self._resolve_simulation_id(simulation_id)
        symbol = symbol.strip()
        data = await self._request(
            "GET",
            f"/api/v1/market/ohlcv/{symbol}",
            params={"days": days, "simulation_id": sim},
        )
        if not isinstance(data, list):
            raise APIError(502, "expected OHLCV response to be a JSON array")
        return data

    async def list_orders(
        self,
        limit: int = 100,
        simulation_id: str | None = None,
    ) -> list[dict[str, Any]]:
        """GET /api/v1/orders/{agent_id}?simulation_id=&limit=..."""
        sim = self._resolve_simulation_id(simulation_id)
        data = await self._request(
            "GET",
            f"/api/v1/orders/{self.agent_id}",
            params={"simulation_id": sim, "limit": limit},
        )
        if not isinstance(data, list):
            raise APIError(502, "expected orders response to be a JSON array")
        return data

    async def place_order(
        self,
        order: Order,
        simulation_id: str | None = None,
    ) -> dict[str, Any]:
        """POST /api/v1/orders with agent_id + simulation_id."""
        sim = self._resolve_simulation_id(simulation_id)
        payload = {
            "simulation_id": sim,
            "agent_id": self.agent_id,
            **order.model_dump(exclude_none=True),
        }
        return await self._request("POST", "/api/v1/orders", json=payload)

    async def cancel_order(
        self,
        order_id: str,
        simulation_id: str | None = None,
    ) -> dict[str, Any]:
        """DELETE /api/v1/orders/{order_id}?simulation_id=..."""
        sim = self._resolve_simulation_id(simulation_id)
        return await self._request(
            "DELETE",
            f"/api/v1/orders/{order_id}",
            params={"simulation_id": sim},
        )
