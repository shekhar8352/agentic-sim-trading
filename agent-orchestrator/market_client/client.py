"""HTTP client for the Go market-simulator API (full implementation: roadmap Step 12)."""

from __future__ import annotations

import httpx
from pydantic import BaseModel, Field


class Order(BaseModel):
    symbol: str
    side: str  # buy | sell
    order_type: str = "market"
    quantity: int = Field(gt=0)
    price: float | None = None


class MarketClient:
    def __init__(self, base_url: str, agent_id: str, api_key: str):
        self.base_url = base_url.rstrip("/")
        self.agent_id = agent_id
        self.headers = {"X-Agent-ID": agent_id, "X-API-Key": api_key}

    def _params_sim(self, simulation_id: str) -> dict[str, str]:
        return {"simulation_id": simulation_id}

    async def get_portfolio(self, simulation_id: str) -> dict:
        async with httpx.AsyncClient() as client:
            r = await client.get(
                f"{self.base_url}/api/v1/portfolio/{self.agent_id}",
                headers=self.headers,
                params=self._params_sim(simulation_id),
            )
            r.raise_for_status()
            return r.json()

    async def get_ohlcv(self, symbol: str, simulation_id: str, days: int = 30) -> list[dict]:
        async with httpx.AsyncClient() as client:
            r = await client.get(
                f"{self.base_url}/api/v1/market/ohlcv/{symbol}",
                headers=self.headers,
                params={"days": days, **self._params_sim(simulation_id)},
            )
            r.raise_for_status()
            return r.json()

    async def place_order(self, simulation_id: str, order: Order) -> dict:
        payload = {
            "simulation_id": simulation_id,
            "agent_id": self.agent_id,
            **order.model_dump(exclude_none=True),
        }
        async with httpx.AsyncClient() as client:
            r = await client.post(
                f"{self.base_url}/api/v1/orders",
                json=payload,
                headers=self.headers,
            )
            r.raise_for_status()
            return r.json()
