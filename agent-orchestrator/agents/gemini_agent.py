from __future__ import annotations

import asyncio
import os
from typing import Any

from agents.llm_agent import LLMAgent


class GeminiAgent(LLMAgent):
    """Google Gemini-backed agent (requires pip install -e '.[llm]')."""

    def __init__(self, agent_id: str, sim_id: str, config: dict[str, Any]):
        super().__init__(agent_id, sim_id, config)
        try:
            import google.generativeai as genai
        except ImportError as exc:
            raise ImportError(
                "GeminiAgent requires google-generativeai; install with: pip install -e '.[llm]'"
            ) from exc

        api_key = config.get("google_api_key") or os.getenv("GOOGLE_API_KEY")
        if not api_key:
            raise ValueError("GOOGLE_API_KEY or config google_api_key required for GeminiAgent")
        genai.configure(api_key=api_key)
        self.model = config.get("model", "gemini-2.0-flash")
        self._genai = genai
        self._model = genai.GenerativeModel(
            self.model,
            system_instruction=self.system_prompt,
        )

    async def complete(self, context: str) -> str:
        response = await asyncio.to_thread(self._model.generate_content, context)
        return response.text or "[]"
