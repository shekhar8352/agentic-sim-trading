from __future__ import annotations

from typing import Any

from agents.base_agent import BaseAgent


class CustomAgent(BaseAgent):
    """Rule-based / technical-analysis agent (Step 13)."""

    async def build_context(self, current_date: str) -> str:
        raise NotImplementedError("CustomAgent.build_context — implement in Step 13")

    async def decide(self, context: str) -> dict[str, Any]:
        raise NotImplementedError("CustomAgent.decide — implement in Step 13")
