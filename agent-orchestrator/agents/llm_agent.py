from __future__ import annotations

import logging
from abc import abstractmethod

from agents.base_agent import BaseAgent
from agents.decisions import parse_orders_json
from market_client.models import Order
from prompts.system import TRADING_SYSTEM_PROMPT

logger = logging.getLogger(__name__)


class LLMAgent(BaseAgent):
    """Base class for LLM-backed agents with shared decide() parsing."""

    system_prompt: str = TRADING_SYSTEM_PROMPT

    @abstractmethod
    async def complete(self, context: str) -> str:
        """Return raw model text (expected to contain a JSON order array)."""

    async def decide(self, context: str) -> list[Order]:
        try:
            raw = await self.complete(context)
            return parse_orders_json(raw)
        except Exception as exc:
            logger.warning("agent=%s llm decide error: %s", self.agent_id, exc)
            return []
