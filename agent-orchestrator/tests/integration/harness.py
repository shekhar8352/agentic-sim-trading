from __future__ import annotations

from dataclasses import dataclass
from typing import Any

import httpx

__all__ = ["RegisteredAgent", "bootstrap_three_agent_simulation"]


@dataclass
class RegisteredAgent:
    agent_id: str
    api_key: str
    name: str
    provider_key: str  # YAML provider string
    model: str


async def _post_json(
    client: httpx.AsyncClient,
    method: str,
    path: str,
    **kw: Any,
) -> dict[str, Any]:
    r = await client.request(method, path, **kw)
    r.raise_for_status()
    if not r.content.strip():
        return {}
    body = r.json()
    assert isinstance(body, dict)
    return body


async def bootstrap_three_agent_simulation(
    *,
    base_url: str,
    redis_url: str,
    start_date: str,
    end_date: str,
    simulation_name: str = "phase4-integration",
    ollama_model: str = "gemma4:latest",
) -> tuple[str, list[RegisteredAgent]]:
    """Create simulation, register Ollama + two rule-based agents, start clock.

    Returns ``(simulation_id, agents)`` with plain API keys suitable for YAML / ``create_agent``.
    """
    base = base_url.rstrip("/")

    async with httpx.AsyncClient(base_url=base, timeout=60.0) as client:
        sim = await _post_json(
            client,
            "POST",
            "/api/v1/simulations",
            json={
                "name": simulation_name,
                "start_date": start_date,
                "end_date": end_date,
            },
        )
        simulation_id = str(sim["id"])

        agents_spec = [
            ("ollama-integration", "ollama", ollama_model),
            ("rules-alpha", "custom", "momentum-v1"),
            ("rules-beta", "custom", "momentum-v1"),
        ]

        registered: list[RegisteredAgent] = []

        for name, provider_key, model in agents_spec:
            row = await _post_json(
                client,
                "POST",
                f"/api/v1/simulations/{simulation_id}/agents",
                json={"name": name, "model": model},
            )
            aid = row["agent_id"]
            api_key = row["api_key"]
            registered.append(
                RegisteredAgent(
                    agent_id=str(aid),
                    api_key=str(api_key),
                    name=name,
                    provider_key=provider_key,
                    model=model,
                )
            )

        await _post_json(client, "POST", f"/api/v1/simulations/{simulation_id}/start")

    _ = redis_url  # callers may subscribe separately; retained for CLI symmetry
    return simulation_id, registered
