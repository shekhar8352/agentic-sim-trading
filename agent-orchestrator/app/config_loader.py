from __future__ import annotations

from pathlib import Path
from typing import Any

import yaml
from pydantic import BaseModel, Field


class AgentEntry(BaseModel):
    name: str
    provider: str  # claude | gpt | gemini | custom
    model: str
    agent_id: str | None = None
    api_key: str | None = None


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
