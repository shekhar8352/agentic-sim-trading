from __future__ import annotations

from typing import Any

import httpx

from agents.llm_agent import LLMAgent


class OllamaAgent(LLMAgent):
    """Trading agent backed by a local **Ollama** HTTP API (``/api/chat``)."""

    def __init__(self, agent_id: str, sim_id: str, config: dict[str, Any]):
        super().__init__(agent_id, sim_id, config)
        base = str(config.get("ollama_base_url", "http://127.0.0.1:11434")).rstrip("/")
        self._ollama_base = base
        self.model = str(config.get("model") or "gemma3:270m")
        self._timeout = float(config.get("ollama_timeout_seconds", 120))

    async def complete(self, context: str) -> str:
        url = f"{self._ollama_base}/api/chat"
        payload: dict[str, Any] = {
            "model": self.model,
            "messages": [
                {"role": "system", "content": self.system_prompt},
                {"role": "user", "content": context},
            ],
            "stream": False,
        }
        async with httpx.AsyncClient(timeout=self._timeout) as client:
            response = await client.post(url, json=payload)
            response.raise_for_status()
            data = response.json()
        msg = data.get("message") or {}
        return str(msg.get("content") or "")
