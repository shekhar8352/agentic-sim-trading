from __future__ import annotations

import os
from typing import Any

from agents.base_agent import BaseAgent
from agents.claude_agent import ClaudeAgent
from agents.custom_agent import CustomAgent
from agents.gemini_agent import GeminiAgent
from agents.gpt_agent import GPTAgent
from agents.ollama_agent import OllamaAgent
from agents.strategies import CUSTOM_STRATEGY_MODELS
from agents.team_agent import wrap_with_team
from app.config_loader import AgentEntry
from prompts.roles import DEFAULT_TEAM_ROLES


def _as_dict(entry: AgentEntry | dict[str, Any]) -> dict[str, Any]:
    if isinstance(entry, AgentEntry):
        return entry.model_dump()
    return dict(entry)


def _wants_team_mode(data: dict[str, Any], provider: str) -> bool:
    """LLM desks default to multi-role team mode; custom strategies stay single-agent."""
    if provider == "custom":
        return False
    if "team_mode" in data and data["team_mode"] is not None:
        return bool(data["team_mode"])
    # Explicit team block implies enabled unless team_mode=false.
    if isinstance(data.get("team"), dict):
        return bool(data["team"].get("enabled", True))
    # New default for LLM providers: analyst → risk/strategist → head.
    return True


def _team_roles(data: dict[str, Any]) -> list[str] | None:
    team = data.get("team")
    if isinstance(team, dict) and team.get("roles"):
        return list(team["roles"])
    roles = data.get("team_roles")
    if roles:
        return list(roles)
    return list(DEFAULT_TEAM_ROLES)


def _agent_config(entry: AgentEntry | dict[str, Any], go_service_url: str) -> dict[str, Any]:
    data = _as_dict(entry)
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
        "personality": data.get("personality"),
        "strategy": data.get("strategy") or data.get("model"),
        "team_mode": data.get("team_mode"),
        "team": data.get("team"),
        "team_roles": data.get("team_roles"),
        "ollama_base_url": (
            (data.get("ollama_base_url") or os.environ.get("OLLAMA_BASE_URL", "")).rstrip("/")
            or "http://127.0.0.1:11434"
        ),
        "ollama_timeout_seconds": float(data.get("ollama_timeout_seconds") or 120),
    }


def _create_backend(
    agent_id: str,
    sim_id: str,
    config: dict[str, Any],
) -> BaseAgent:
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
        strategy = str(config.get("strategy") or "momentum-v1").lower()
        agent_cls = CUSTOM_STRATEGY_MODELS.get(strategy, CustomAgent)
        return agent_cls(agent_id, sim_id, config)
    raise ValueError(f"unknown agent provider: {provider}")


def create_agent(
    agent_id: str,
    sim_id: str,
    entry: AgentEntry | dict[str, Any],
    go_service_url: str,
) -> BaseAgent:
    """Instantiate an agent from config/agents.yaml entry.

    LLM providers are wrapped in a multi-role ``TradingTeamAgent`` by default
    (analyst, risk officer, strategist, head). Set ``team_mode: false`` for the
    legacy single-prompt trader.
    """
    data = _as_dict(entry)
    config = _agent_config(entry, go_service_url)
    provider = str(config.get("provider", "custom")).lower()
    backend = _create_backend(agent_id, sim_id, config)
    return wrap_with_team(
        backend,
        team_mode=_wants_team_mode(data, provider),
        roles=_team_roles(data),
    )
