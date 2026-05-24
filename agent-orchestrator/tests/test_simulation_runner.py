from __future__ import annotations

import asyncio
from unittest.mock import AsyncMock, MagicMock

import pytest

from orchestrator.runner import SimulationRunner


@pytest.mark.asyncio
async def test_dispatch_skips_tick_for_other_simulation():
    agent = MagicMock()
    agent.agent_id = "a1"
    agent.run_turn = AsyncMock(return_value=[])

    runner = SimulationRunner([agent], "redis://localhost", simulation_id="sim-A")
    await runner.dispatch_event(
        {"event": "sim.tick", "simulation_id": "sim-B", "date": "2024-06-01"}
    )
    agent.run_turn.assert_not_called()


@pytest.mark.asyncio
async def test_dispatch_runs_tick_when_sim_matches():
    agent = MagicMock()
    agent.agent_id = "a1"
    agent.run_turn = AsyncMock(return_value=[])

    runner = SimulationRunner([agent], "redis://localhost", simulation_id="sim-A")
    await runner.dispatch_event(
        {"event": "sim.tick", "simulation_id": "sim-A", "date": "2024-06-01"}
    )
    agent.run_turn.assert_awaited_once_with("2024-06-01")


@pytest.mark.asyncio
async def test_handle_tick_runs_agents_concurrently():
    """Second agent starts before first finishes ⇒ gather ran concurrently."""
    order: list[str] = []

    async def run_turn_1(date: str):
        order.append("start-1")
        await asyncio.sleep(0.03)
        order.append("end-1")
        return []

    async def run_turn_2(date: str):
        order.append("start-2")
        await asyncio.sleep(0.03)
        order.append("end-2")
        return []

    a1 = MagicMock()
    a1.agent_id = "1"
    a1.run_turn = AsyncMock(side_effect=run_turn_1)

    a2 = MagicMock()
    a2.agent_id = "2"
    a2.run_turn = AsyncMock(side_effect=run_turn_2)

    runner = SimulationRunner([a1, a2], "redis://localhost", simulation_id=None)
    await runner.handle_tick({"simulation_id": "x", "date": "2024-01-02"})

    assert order.index("start-2") < order.index("end-1")


@pytest.mark.asyncio
async def test_completion_sets_stop_when_filtered():
    runner = SimulationRunner(
        [], "redis://localhost", simulation_id="sim-A", stop_on_completed=True
    )
    assert not runner._stop_requested
    await runner.dispatch_event({"event": "sim.completed", "simulation_id": "sim-A"})
    assert runner._stop_requested


@pytest.mark.asyncio
async def test_completion_does_not_stop_for_other_sim_when_filtered():
    runner = SimulationRunner(
        [], "redis://localhost", simulation_id="sim-A", stop_on_completed=True
    )
    await runner.dispatch_event({"event": "sim.completed", "simulation_id": "sim-B"})
    assert not runner._stop_requested
