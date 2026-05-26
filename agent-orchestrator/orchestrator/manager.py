from __future__ import annotations

import asyncio
import logging
from typing import Any

from agents.base_agent import BaseAgent
from agents.factory import create_agent
from orchestrator.runner import SimulationRunner

logger = logging.getLogger(__name__)


class OrchestratorManager:
    """Tracks background SimulationRunner tasks keyed by simulation_id."""

    def __init__(self) -> None:
        self._tasks: dict[str, asyncio.Task[None]] = {}
        self._runners: dict[str, SimulationRunner] = {}
        self._agents: dict[str, list[BaseAgent]] = {}

    def is_running(self, simulation_id: str) -> bool:
        task = self._tasks.get(simulation_id)
        return task is not None and not task.done()

    def get_agents(self, simulation_id: str) -> list[BaseAgent]:
        return list(self._agents.get(simulation_id, []))

    async def start_simulation(
        self,
        simulation_id: str,
        agents: list[BaseAgent],
        *,
        redis_url: str,
        redis_reconnect_base_seconds: float,
        redis_reconnect_max_delay_seconds: float,
    ) -> None:
        if self.is_running(simulation_id):
            raise ValueError(f"orchestrator already running for simulation {simulation_id}")

        runner = SimulationRunner(
            agents,
            redis_url,
            simulation_id=simulation_id,
            redis_reconnect_base_seconds=redis_reconnect_base_seconds,
            redis_reconnect_max_delay_seconds=redis_reconnect_max_delay_seconds,
        )
        self._runners[simulation_id] = runner
        self._agents[simulation_id] = agents

        async def _run() -> None:
            try:
                await runner.start()
            except asyncio.CancelledError:
                raise
            except Exception:
                logger.exception("orchestrator task failed simulation_id=%s", simulation_id)
            finally:
                self._tasks.pop(simulation_id, None)
                self._runners.pop(simulation_id, None)

        self._tasks[simulation_id] = asyncio.create_task(_run())
        logger.info("orchestrator started simulation_id=%s agents=%d", simulation_id, len(agents))

    async def stop_simulation(self, simulation_id: str) -> None:
        runner = self._runners.get(simulation_id)
        if runner is not None:
            runner._stop_requested = True
        task = self._tasks.get(simulation_id)
        if task is not None and not task.done():
            task.cancel()
            try:
                await task
            except asyncio.CancelledError:
                pass
        self._tasks.pop(simulation_id, None)
        self._runners.pop(simulation_id, None)
        self._agents.pop(simulation_id, None)

    async def stop_all(self) -> None:
        for sim_id in list(self._tasks.keys()):
            await self.stop_simulation(sim_id)

    def update_agent_prompts(self, simulation_id: str, prompts_by_agent_id: dict[str, str]) -> int:
        updated = 0
        for agent in self._agents.get(simulation_id, []):
            if agent.agent_id not in prompts_by_agent_id:
                continue
            if hasattr(agent, "system_prompt"):
                agent.system_prompt = prompts_by_agent_id[agent.agent_id]
                updated += 1
        return updated


_manager: OrchestratorManager | None = None


def get_manager() -> OrchestratorManager:
    global _manager
    if _manager is None:
        _manager = OrchestratorManager()
    return _manager
