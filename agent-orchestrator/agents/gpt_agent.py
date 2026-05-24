from __future__ import annotations

import os
from typing import Any

from openai import AsyncOpenAI

from agents.decisions import resolve_gpt_model
from agents.llm_agent import LLMAgent


class GPTAgent(LLMAgent):
    """OpenAI GPT-backed trading agent."""

    def __init__(self, agent_id: str, sim_id: str, config: dict[str, Any]):
        super().__init__(agent_id, sim_id, config)
        api_key = config.get("openai_api_key") or os.getenv("OPENAI_API_KEY")
        self.model = resolve_gpt_model(str(config.get("model") or "gpt-4o"))
        self.llm = AsyncOpenAI(api_key=api_key) if api_key else AsyncOpenAI()

    def _completion_params(self) -> dict[str, Any]:
        token_limit = int(self.config.get("max_tokens", 1000))
        params: dict[str, Any] = {
            "model": self.model,
            "messages": [
                {"role": "system", "content": self.system_prompt},
                {"role": "user", "content": ""},
            ],
        }
        # GPT-5 / o-series models use max_completion_tokens on Chat Completions.
        if self.model.startswith(("gpt-5", "o1", "o3", "o4")):
            params["max_completion_tokens"] = token_limit
        else:
            params["max_tokens"] = token_limit
        return params

    async def complete(self, context: str) -> str:
        params = self._completion_params()
        params["messages"][1]["content"] = context
        response = await self.llm.chat.completions.create(**params)
        return response.choices[0].message.content or "[]"
