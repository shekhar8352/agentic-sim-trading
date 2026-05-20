from __future__ import annotations

from typing import Any

from agents.base_agent import BaseAgent


class GPTAgent(BaseAgent):
    """OpenAI GPT-backed agent (Step 13)."""

    async def build_context(self, current_date: str) -> str:
        raise NotImplementedError("GPTAgent.build_context — implement in Step 13")

    async def decide(self, context: str) -> dict[str, Any]:
        raise NotImplementedError("GPTAgent.decide — implement in Step 13")
