from __future__ import annotations

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from app.launch_service import LaunchError, launch_simulation, update_simulation_prompts
from app.providers import default_system_prompt, list_providers, list_strategies, team_desk_catalog
from app.settings import get_settings
from orchestrator.manager import get_manager
from prompts.personalities import DEFAULT_PERSONALITY_ID, build_system_prompt, list_personalities
from prompts.roles import DEFAULT_TEAM_ROLES

router = APIRouter(prefix="/api/v1")


class LaunchAgentSpec(BaseModel):
    name: str = Field(min_length=1, max_length=100)
    provider: str
    model: str
    personality: str = DEFAULT_PERSONALITY_ID
    system_prompt: str | None = None
    # Multi-agent desk (LLM only). Default true — analyst/risk/strategist → head.
    team_mode: bool | None = None
    team_roles: list[str] | None = None


class LaunchSimulationRequest(BaseModel):
    name: str = Field(min_length=1, max_length=100)
    start_date: str
    end_date: str
    checkpoint_interval_days: int = Field(default=5, ge=1, le=60)
    agents: list[LaunchAgentSpec] = Field(min_length=1, max_length=10)


class PromptUpdate(BaseModel):
    agent_id: str
    system_prompt: str


class UpdatePromptsRequest(BaseModel):
    agents: list[PromptUpdate]


@router.get("/providers")
def get_providers():
    settings = get_settings()
    desk = team_desk_catalog()
    return {
        "providers": list_providers(settings),
        "default_system_prompt": default_system_prompt(),
        "personalities": list_personalities(),
        "strategies": list_strategies(),
        "default_personality": DEFAULT_PERSONALITY_ID,
        "team_desk": desk,
        "default_team_mode": desk["default_team_mode"],
        "default_team_roles": list(DEFAULT_TEAM_ROLES),
    }


@router.get("/personalities")
def get_personalities():
    return {
        "personalities": list_personalities(),
        "default_personality": DEFAULT_PERSONALITY_ID,
        "default_system_prompt": default_system_prompt(),
    }


@router.get("/personalities/{personality_id}/prompt")
def get_personality_prompt(personality_id: str):
    try:
        prompt = build_system_prompt(personality_id)
    except ValueError as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    return {"personality": personality_id, "system_prompt": prompt}


@router.get("/orchestrator/status/{simulation_id}")
def orchestrator_status(simulation_id: str):
    manager = get_manager()
    return {
        "simulation_id": simulation_id,
        "running": manager.is_running(simulation_id),
        "agent_count": len(manager.get_agents(simulation_id)),
    }


@router.post("/simulations/launch")
async def post_launch_simulation(body: LaunchSimulationRequest):
    settings = get_settings()
    try:
        result = await launch_simulation(
            settings,
            name=body.name,
            start_date=body.start_date,
            end_date=body.end_date,
            checkpoint_interval_days=body.checkpoint_interval_days,
            agent_specs=[a.model_dump() for a in body.agents],
        )
    except LaunchError as exc:
        raise HTTPException(status_code=exc.status_code, detail=str(exc)) from exc
    return result


@router.patch("/simulations/{simulation_id}/prompts")
async def patch_simulation_prompts(simulation_id: str, body: UpdatePromptsRequest):
    settings = get_settings()
    try:
        return await update_simulation_prompts(
            settings,
            simulation_id,
            [a.model_dump() for a in body.agents],
        )
    except LaunchError as exc:
        raise HTTPException(status_code=exc.status_code, detail=str(exc)) from exc
