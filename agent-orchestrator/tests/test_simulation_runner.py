from __future__ import annotations

import asyncio
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
import redis.asyncio as redis

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


@pytest.mark.asyncio
async def test_handle_tick_isolates_failing_agent():
    ok = MagicMock()
    ok.agent_id = "ok"
    ok.run_turn = AsyncMock(return_value=[])

    bad = MagicMock()
    bad.agent_id = "bad"
    bad.run_turn = AsyncMock(side_effect=RuntimeError("boom"))

    runner = SimulationRunner([ok, bad], "redis://localhost")
    await runner.handle_tick({"simulation_id": "x", "date": "2024-01-02"})

    ok.run_turn.assert_awaited_once_with("2024-01-02")
    bad.run_turn.assert_awaited_once_with("2024-01-02")


@pytest.mark.asyncio
async def test_dispatch_event_survives_handler_error():
    agent = MagicMock()
    agent.agent_id = "a1"
    agent.run_turn = AsyncMock(side_effect=ValueError("handler broke"))

    runner = SimulationRunner([agent], "redis://localhost", simulation_id="sim-A")
    await runner.dispatch_event(
        {"event": "sim.tick", "simulation_id": "sim-A", "date": "2024-06-01"}
    )
    agent.run_turn.assert_awaited_once()


@pytest.mark.asyncio
async def test_dispatch_handles_sim_resumed():
    runner = SimulationRunner([], "redis://localhost", simulation_id="sim-A")
    await runner.dispatch_event(
        {"event": "sim.resumed", "simulation_id": "sim-A", "date": "2024-03-01"}
    )


@pytest.mark.asyncio
async def test_start_reconnects_after_redis_disconnect():
    runner = SimulationRunner([], "redis://localhost")
    attempts = {"n": 0}
    sleeps: list[float] = []

    async def flaky_listen():
        attempts["n"] += 1
        if attempts["n"] == 1:
            raise redis.ConnectionError("gone")
        runner._stop_requested = True

    async def mock_sleep(delay: float) -> None:
        sleeps.append(delay)

    runner._listen_once = flaky_listen  # type: ignore[method-assign]

    with patch("orchestrator.runner.asyncio.sleep", mock_sleep):
        await runner.start()

    assert attempts["n"] == 2
    assert sleeps == [1.0]
