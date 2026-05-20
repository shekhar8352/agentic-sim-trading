from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Any

from market_client.client import MarketClient


class BaseAgent(ABC):
    """Abstract trading agent; LLM and rule-based subclasses implement decide()."""

    def __init__(self, agent_id: str, sim_id: str, config: dict[str, Any]):
        self.agent_id = agent_id
        self.sim_id = sim_id
        self.config = config
        self.client = MarketClient(
            config["go_service_url"],
            agent_id,
            config["api_key"],
        )

    @abstractmethod
    async def build_context(self, current_date: str) -> str:
        """Gather portfolio and market data for the model (Step 13)."""

    @abstractmethod
    async def decide(self, context: str) -> dict[str, Any]:
        """Return a trade decision payload (symbol, side, quantity, etc.)."""

    async def act(self, current_date: str) -> dict[str, Any]:
        context = await self.build_context(current_date)
        decision = await self.decide(context)
        return decision
