from __future__ import annotations

from typing import Any

from agents.base_agent import BaseAgent


class ClaudeAgent(BaseAgent):
    """Anthropic Claude-backed agent (Step 13)."""

    async def build_context(self, current_date: str) -> str:
        raise NotImplementedError("ClaudeAgent.build_context — implement in Step 13")

    async def decide(self, context: str) -> dict[str, Any]:
        raise NotImplementedError("ClaudeAgent.decide — implement in Step 13")
