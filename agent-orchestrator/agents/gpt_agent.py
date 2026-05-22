from __future__ import annotations

import os
from typing import Any

from openai import AsyncOpenAI

from agents.llm_agent import LLMAgent


class GPTAgent(LLMAgent):
    """OpenAI GPT-backed trading agent."""

    def __init__(self, agent_id: str, sim_id: str, config: dict[str, Any]):
        super().__init__(agent_id, sim_id, config)
        api_key = config.get("openai_api_key") or os.getenv("OPENAI_API_KEY")
        self.model = config.get("model", "gpt-4o")
        self.llm = AsyncOpenAI(api_key=api_key) if api_key else AsyncOpenAI()

    async def complete(self, context: str) -> str:
        response = await self.llm.chat.completions.create(
            model=self.model,
            max_tokens=int(self.config.get("max_tokens", 1000)),
            messages=[
                {"role": "system", "content": self.system_prompt},
                {"role": "user", "content": context},
            ],
        )
        return response.choices[0].message.content or "[]"
