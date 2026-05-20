from __future__ import annotations

from typing import Any

from agents.base_agent import BaseAgent


class GeminiAgent(BaseAgent):
    """Google Gemini-backed agent (Step 13; requires pip install -e '.[llm]')."""

    async def build_context(self, current_date: str) -> str:
        raise NotImplementedError("GeminiAgent.build_context — implement in Step 13")

    async def decide(self, context: str) -> dict[str, Any]:
        raise NotImplementedError("GeminiAgent.decide — implement in Step 13")
