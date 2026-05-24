from __future__ import annotations

import os
from typing import Any

from agents.base_agent import BaseAgent
from agents.claude_agent import ClaudeAgent
from agents.custom_agent import CustomAgent
from agents.gemini_agent import GeminiAgent
from agents.gpt_agent import GPTAgent
from agents.ollama_agent import OllamaAgent
from app.config_loader import AgentEntry


def _agent_config(entry: AgentEntry | dict[str, Any], go_service_url: str) -> dict[str, Any]:
    if isinstance(entry, AgentEntry):
        data = entry.model_dump()
    else:
        data = dict(entry)
    agent_id = data.get("agent_id")
    api_key = data.get("api_key")

    if not agent_id:
        raise ValueError(f"agent {data.get('name', '?')} missing agent_id in config")

    if not api_key:
        raise ValueError(
            f"agent {data.get('name', '?')} missing api_key "
            "(use key from simulator registration)"
        )

    return {
        "go_service_url": go_service_url,
        "api_key": api_key,
        "model": data.get("model"),
        "name": data.get("name"),
        "provider": data.get("provider"),
        "anthropic_api_key": data.get("anthropic_api_key"),
        "openai_api_key": data.get("openai_api_key"),
        "google_api_key": data.get("google_api_key"),
        "max_tokens": data.get("max_tokens", 1000),
        "context_symbol_count": data.get("context_symbol_count", 10),
        "system_prompt": data.get("system_prompt"),
        "ollama_base_url": (
            (data.get("ollama_base_url") or os.environ.get("OLLAMA_BASE_URL", "")).rstrip("/")
            or "http://127.0.0.1:11434"
        ),
        "ollama_timeout_seconds": float(data.get("ollama_timeout_seconds") or 120),
    }


def create_agent(
    agent_id: str,
    sim_id: str,
    entry: AgentEntry | dict[str, Any],
    go_service_url: str,
) -> BaseAgent:
    """Instantiate an agent from config/agents.yaml entry."""
    config = _agent_config(entry, go_service_url)
    provider = str(config.get("provider", "custom")).lower()

    if provider == "claude":
        return ClaudeAgent(agent_id, sim_id, config)
    if provider in ("gpt", "openai"):
        return GPTAgent(agent_id, sim_id, config)
    if provider == "gemini":
        return GeminiAgent(agent_id, sim_id, config)
    if provider == "ollama":
        return OllamaAgent(agent_id, sim_id, config)
    if provider == "custom":
        return CustomAgent(agent_id, sim_id, config)
    raise ValueError(f"unknown agent provider: {provider}")
