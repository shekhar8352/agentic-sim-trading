from __future__ import annotations

import os
from typing import Any

from anthropic import AsyncAnthropic

from agents.llm_agent import LLMAgent


class ClaudeAgent(LLMAgent):
    """Anthropic Claude-backed trading agent."""

    def __init__(self, agent_id: str, sim_id: str, config: dict[str, Any]):
        super().__init__(agent_id, sim_id, config)
        api_key = config.get("anthropic_api_key") or os.getenv("ANTHROPIC_API_KEY")
        self.model = config.get("model", "claude-sonnet-4-20250514")
        self.llm = AsyncAnthropic(api_key=api_key) if api_key else AsyncAnthropic()

    async def complete(self, context: str) -> str:
        response = await self.llm.messages.create(
            model=self.model,
            max_tokens=int(self.config.get("max_tokens", 1000)),
            system=self.system_prompt,
            messages=[{"role": "user", "content": context}],
        )
        return response.content[0].text
