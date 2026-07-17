from __future__ import annotations

import logging
from typing import Any

import httpx

from agents.decisions import resolve_gpt_model
from agents.factory import create_agent
from agents.team_agent import TradingTeamAgent
from app.providers import list_providers
from app.settings import Settings
from orchestrator.manager import get_manager
from prompts.personalities import build_system_prompt, get_personality
from prompts.roles import DEFAULT_TEAM_ROLES, normalize_team_roles

logger = logging.getLogger(__name__)


class LaunchError(Exception):
    def __init__(self, message: str, *, status_code: int = 400):
        super().__init__(message)
        self.status_code = status_code


async def _go_json(
    client: httpx.AsyncClient,
    method: str,
    path: str,
    *,
    json_body: dict[str, Any] | None = None,
) -> dict[str, Any]:
    resp = await client.request(method, path, json=json_body)
    if resp.status_code >= 400:
        detail = resp.text
        try:
            body = resp.json()
            detail = body.get("detail", detail)
        except Exception:
            pass
        raise LaunchError(
            f"market-simulator {method} {path}: {detail}",
            status_code=resp.status_code,
        )
    if not resp.content:
        return {}
    data = resp.json()
    return data if isinstance(data, dict) else {"data": data}


async def launch_simulation(
    settings: Settings,
    *,
    name: str,
    start_date: str,
    end_date: str,
    checkpoint_interval_days: int,
    agent_specs: list[dict[str, Any]],
) -> dict[str, Any]:
    if not settings.redis_url:
        raise LaunchError("REDIS_URL is not configured on the orchestrator", status_code=503)
    if not agent_specs:
        raise LaunchError("At least one agent is required")

    availability = {p["id"]: p["available"] for p in list_providers(settings)}
    models_by_provider = {p["id"]: set(p["models"]) for p in list_providers(settings)}
    for spec in agent_specs:
        provider = spec["provider"]
        if not availability.get(provider):
            raise LaunchError(f"provider '{provider}' is not available (check API keys in .env)")
        model = spec.get("model")
        if provider in ("gpt", "openai") and model:
            model = resolve_gpt_model(model)
            spec["model"] = model
        if model and model not in models_by_provider.get(provider, set()):
            raise LaunchError(f"model '{model}' is not supported for provider '{provider}'")
        if provider != "custom":
            try:
                get_personality(spec.get("personality"))
            except ValueError as exc:
                raise LaunchError(str(exc)) from exc
            if spec.get("team_roles") is not None:
                try:
                    normalize_team_roles(spec.get("team_roles"))
                except ValueError as exc:
                    raise LaunchError(str(exc)) from exc

    go_base = settings.market_simulator_url.rstrip("/")
    config_fragment = {
        "checkpoint_interval_days": checkpoint_interval_days,
        "orchestrator_agents": [],
    }

    registered: list[dict[str, Any]] = []
    agent_instances = []

    async with httpx.AsyncClient(base_url=go_base, timeout=60.0) as client:
        sim = await _go_json(
            client,
            "POST",
            "/api/v1/simulations",
            json_body={
                "name": name,
                "start_date": start_date,
                "end_date": end_date,
                "config": {"checkpoint_interval_days": checkpoint_interval_days},
            },
        )
        sim_id = sim.get("id")
        if not sim_id:
            raise LaunchError("simulation create did not return id", status_code=502)

        for spec in agent_specs:
            reg = await _go_json(
                client,
                "POST",
                f"/api/v1/simulations/{sim_id}/agents",
                json_body={"name": spec["name"], "model": spec["model"]},
            )
            agent_id = reg.get("agent_id")
            api_key = reg.get("api_key")
            if not agent_id or not api_key:
                raise LaunchError("agent registration missing agent_id or api_key", status_code=502)

            personality = spec.get("personality")
            is_custom = spec["provider"] == "custom"
            if is_custom:
                prompt = ""
                team_mode = False
                team_roles: list[str] | None = None
            else:
                prompt = build_system_prompt(
                    personality,
                    custom_override=spec.get("system_prompt"),
                )
                # Default LLM desks to multi-role team mode.
                team_mode = True if spec.get("team_mode") is None else bool(spec.get("team_mode"))
                raw_roles = spec.get("team_roles")
                team_roles = (
                    list(normalize_team_roles(raw_roles))
                    if raw_roles is not None
                    else list(DEFAULT_TEAM_ROLES)
                )
            entry = {
                "name": spec["name"],
                "provider": spec["provider"],
                "model": spec["model"],
                "agent_id": agent_id,
                "api_key": api_key,
                "system_prompt": prompt,
                "personality": personality,
                "strategy": spec["model"] if is_custom else None,
                "team_mode": team_mode,
                "team_roles": team_roles,
            }
            agent = create_agent(agent_id, sim_id, entry, go_base)
            agent_instances.append(agent)
            roles_out = (
                list(agent.roles)
                if isinstance(agent, TradingTeamAgent)
                else None
            )
            registered.append(
                {
                    "agent_id": agent_id,
                    "name": spec["name"],
                    "provider": spec["provider"],
                    "model": spec["model"],
                    "personality": personality,
                    "system_prompt": prompt or None,
                    "team_mode": isinstance(agent, TradingTeamAgent),
                    "team_roles": roles_out,
                }
            )
            config_fragment["orchestrator_agents"].append(
                {
                    "agent_id": agent_id,
                    "name": spec["name"],
                    "provider": spec["provider"],
                    "model": spec["model"],
                    "personality": personality,
                    "system_prompt": prompt or None,
                    "team_mode": isinstance(agent, TradingTeamAgent),
                    "team_roles": roles_out,
                }
            )

        await _go_json(
            client,
            "PATCH",
            f"/api/v1/simulations/{sim_id}/config",
            json_body={"config": config_fragment},
        )
        await _go_json(client, "POST", f"/api/v1/simulations/{sim_id}/start")

    manager = get_manager()
    if manager.is_running(sim_id):
        await manager.stop_simulation(sim_id)

    await manager.start_simulation(
        sim_id,
        agent_instances,
        redis_url=settings.redis_url,
        redis_reconnect_base_seconds=settings.redis_reconnect_base_seconds,
        redis_reconnect_max_delay_seconds=settings.redis_reconnect_max_delay_seconds,
    )

    logger.info(
        "launched simulation_id=%s name=%s agents=%d",
        sim_id,
        name,
        len(registered),
    )
    return {
        "simulation_id": sim_id,
        "status": "running",
        "agents": registered,
        "orchestrator_running": True,
    }


async def update_simulation_prompts(
    settings: Settings,
    simulation_id: str,
    updates: list[dict[str, str]],
) -> dict[str, Any]:
    go_base = settings.market_simulator_url.rstrip("/")
    prompts_by_id = {u["agent_id"]: u["system_prompt"] for u in updates if u.get("agent_id")}

    async with httpx.AsyncClient(base_url=go_base, timeout=30.0) as client:
        sim = await _go_json(client, "GET", f"/api/v1/simulations/{simulation_id}")
        config = sim.get("config") or {}
        if isinstance(config, str):
            import json

            config = json.loads(config) if config else {}
        agents_cfg = config.get("orchestrator_agents") or []
        if not isinstance(agents_cfg, list):
            agents_cfg = []
        for row in agents_cfg:
            if not isinstance(row, dict):
                continue
            aid = row.get("agent_id")
            if aid in prompts_by_id:
                row["system_prompt"] = prompts_by_id[aid]
        await _go_json(
            client,
            "PATCH",
            f"/api/v1/simulations/{simulation_id}/config",
            json_body={"config": {"orchestrator_agents": agents_cfg}},
        )

    manager = get_manager()
    live_updated = manager.update_agent_prompts(simulation_id, prompts_by_id)

    return {
        "simulation_id": simulation_id,
        "updated": len(prompts_by_id),
        "live_agents_updated": live_updated,
    }
