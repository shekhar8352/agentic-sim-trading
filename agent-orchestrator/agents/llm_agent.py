from __future__ import annotations

import asyncio
import logging
from abc import abstractmethod

import httpx

from agents.base_agent import BaseAgent
from agents.decisions import is_valid_llm_output, parse_orders_json
from market_client.models import Order
from prompts.system import TRADING_SYSTEM_PROMPT

logger = logging.getLogger(__name__)

_LLM_MAX_ATTEMPTS = 2  # initial call + one retry (Step 16)


def _is_retryable_llm_error(exc: BaseException) -> bool:
    if isinstance(exc, (asyncio.TimeoutError, httpx.TimeoutException)):
        return True
    if isinstance(exc, httpx.TransportError):
        return True
    name = type(exc).__name__.lower()
    return "timeout" in name or "timed out" in str(exc).lower()


class LLMAgent(BaseAgent):
    """Base class for LLM-backed agents with shared decide() parsing."""

    def __init__(self, agent_id: str, sim_id: str, config: dict[str, Any]):
        super().__init__(agent_id, sim_id, config)
        self.system_prompt = config.get("system_prompt") or TRADING_SYSTEM_PROMPT

    @abstractmethod
    async def complete(self, context: str) -> str:
        """Return raw model text (expected to contain a JSON order array)."""

    async def decide(self, context: str) -> list[Order]:
        for attempt in range(1, _LLM_MAX_ATTEMPTS + 1):
            try:
                raw = await self.complete(context)
                orders = parse_orders_json(raw)
                if not orders and raw.strip() and not is_valid_llm_output(raw, orders):
                    logger.warning(
                        "agent=%s llm output unparseable, holding (raw_len=%d)",
                        self.agent_id,
                        len(raw),
                    )
                return orders
            except Exception as exc:
                if attempt < _LLM_MAX_ATTEMPTS and _is_retryable_llm_error(exc):
                    logger.warning(
                        "agent=%s llm timeout/error attempt=%d retrying: %s",
                        self.agent_id,
                        attempt,
                        exc,
                    )
                    continue
                logger.warning("agent=%s llm decide error: %s", self.agent_id, exc)
                return []
        return []
