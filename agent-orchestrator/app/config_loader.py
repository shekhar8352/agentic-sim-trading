from __future__ import annotations

from pathlib import Path
from typing import Any

import yaml
from pydantic import BaseModel, Field


class TeamConfig(BaseModel):
    """Optional multi-role desk configuration for an LLM agent."""

    enabled: bool = True
    roles: list[str] = Field(
        default_factory=lambda: ["analyst", "risk_officer", "strategist", "head"]
    )


class AgentEntry(BaseModel):
    name: str
    provider: str  # claude | gpt | gemini | ollama | custom
    model: str
    agent_id: str | None = None
    api_key: str | None = None
    ollama_base_url: str | None = None
    ollama_timeout_seconds: float | None = None
    system_prompt: str | None = None
    personality: str | None = None
    # Multi-agent desk: analyst → risk/strategist → head (LLM only).
    team_mode: bool | None = None
    team_roles: list[str] | None = None
    team: TeamConfig | None = None


class AgentsConfig(BaseModel):
    go_service_url: str = "http://localhost:8070"
    simulation_id: str | None = None
    agents: list[AgentEntry] = Field(default_factory=list)


def load_agents_config(path: str | Path) -> AgentsConfig:
    p = Path(path)
    if not p.is_file():
        return AgentsConfig()
    raw: dict[str, Any] = yaml.safe_load(p.read_text()) or {}
    return AgentsConfig.model_validate(raw)
