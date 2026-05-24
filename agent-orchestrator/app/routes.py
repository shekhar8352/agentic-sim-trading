from __future__ import annotations

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from app.launch_service import LaunchError, launch_simulation, update_simulation_prompts
from app.providers import default_system_prompt, list_providers
from app.settings import get_settings
from orchestrator.manager import get_manager

router = APIRouter(prefix="/api/v1")


class LaunchAgentSpec(BaseModel):
    name: str = Field(min_length=1, max_length=100)
    provider: str
    model: str
    system_prompt: str | None = None


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
    return {
        "providers": list_providers(settings),
        "default_system_prompt": default_system_prompt(),
    }


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
