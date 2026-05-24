"""Phase 4 — Step 15 integration checklist (opt-in).

Enable::
    RUN_PHASE4_INTEGRATION=1 pytest tests/integration/test_phase4_integration.py -v

Optional live Ollama chat + ``run_turn``::
    RUN_PHASE4_INTEGRATION=1 OLLAMA_CHAT_INTEGRATION=1 pytest tests/integration/ -v -k leaderboard
"""

from __future__ import annotations

import asyncio
import json
import os
from contextlib import suppress

import httpx
import pytest
import redis.asyncio as redis

from agents.factory import create_agent
from app.config_loader import AgentEntry
from tests.integration.harness import bootstrap_three_agent_simulation

pytestmark = pytest.mark.integration


def _require_integration_env() -> None:
    if os.environ.get("RUN_PHASE4_INTEGRATION", "").strip().lower() not in ("1", "true", "yes"):
        pytest.skip("set RUN_PHASE4_INTEGRATION=1 to run Phase 4 integration tests")


def _sim_base() -> str:
    return os.environ.get("MARKET_SIMULATOR_URL", "http://127.0.0.1:8070").rstrip("/")


def _redis_url() -> str:
    url = os.environ.get("REDIS_URL", "").strip()
    if not url:
        pytest.skip("REDIS_URL not set")
    return url


@pytest.mark.asyncio
async def test_stack_health_ok() -> None:
    _require_integration_env()

    async with httpx.AsyncClient(timeout=10.0) as client:
        r = await client.get(f"{_sim_base()}/health")
        assert r.status_code == 200
        body = r.json()
        pg = body.get("postgres") == "ok"
        assert pg, "start Postgres + schema; see infra/docker-compose.yml"
        assert body.get("redis") == "ok", "Redis must be reachable"


@pytest.mark.asyncio
async def test_step15_tick_emits_redis_sim_tick_event() -> None:
    """After bootstrap + POST .../tick, Redis ``sim.events`` carries ``event\":\"sim.tick``."""
    _require_integration_env()

    start = os.environ.get("INTEGRATION_START_DATE", "2024-01-02").strip()
    end = os.environ.get("INTEGRATION_END_DATE", "2024-01-10").strip()
    redis_url = _redis_url()

    incoming: asyncio.Queue[dict[str, object]] = asyncio.Queue()

    async def _listen() -> None:
        conn = redis.from_url(redis_url, decode_responses=True)
        ps = conn.pubsub()
        await ps.subscribe("sim.events")
        try:
            async for raw in ps.listen():
                if raw["type"] != "message":
                    continue
                data_raw = raw.get("data")
                if isinstance(data_raw, str):
                    with suppress(json.JSONDecodeError):
                        decoded = json.loads(data_raw)
                        if isinstance(decoded, dict):
                            await incoming.put(decoded)
        finally:
            await ps.unsubscribe("sim.events")
            await ps.aclose()
            await conn.aclose()

    listen_task = asyncio.create_task(_listen())
    await asyncio.sleep(0.2)

    simulation_id = ""
    try:
        simulation_id, _ = await bootstrap_three_agent_simulation(
            base_url=_sim_base(),
            redis_url=redis_url,
            start_date=start,
            end_date=end,
            ollama_model=os.environ.get("OLLAMA_MODEL", "gemma4:latest"),
        )

        async with httpx.AsyncClient(timeout=60.0) as client:
            tick_r = await client.post(f"{_sim_base()}/api/v1/simulations/{simulation_id}/tick")
            assert tick_r.status_code == 200, tick_r.text
    finally:
        await asyncio.sleep(1.5)
        listen_task.cancel()
        with suppress(asyncio.CancelledError):
            await listen_task

    ours: dict[str, object] | None = None
    while not incoming.empty():
        evt = incoming.get_nowait()
        if (
            evt.get("simulation_id") == simulation_id
            and evt.get("event") == "sim.tick"
        ):
            ours = evt
            break

    assert ours is not None, (
        "no matching sim.tick (check REDIS_URL matches the simulator; same docker network/host)"
    )
    assert ours.get("date")


@pytest.mark.asyncio
async def test_step15_leaderboard_and_three_agents_registered() -> None:
    """After bootstrap, leaderboard returns rows for all portfolios."""
    _require_integration_env()

    redis_url = _redis_url()
    start = os.environ.get("INTEGRATION_START_DATE", "2024-01-02").strip()
    end = os.environ.get("INTEGRATION_END_DATE", "2024-01-10").strip()
    ollama_model = os.environ.get("OLLAMA_MODEL", "gemma4:latest").strip()

    sim_id, agents_reg = await bootstrap_three_agent_simulation(
        base_url=_sim_base(),
        redis_url=redis_url,
        start_date=start,
        end_date=end,
        ollama_model=ollama_model,
    )

    assert len(agents_reg) == 3

    async with httpx.AsyncClient(timeout=30.0) as client:
        lb_r = await client.get(f"{_sim_base()}/api/v1/leaderboard/{sim_id}")
        lb_r.raise_for_status()
        payload = lb_r.json()
        entries = payload.get("entries", [])
        assert isinstance(entries, list)
        assert len(entries) >= 1

    if os.environ.get("OLLAMA_CHAT_INTEGRATION", "").strip().lower() in ("1", "true", "yes"):
        ollama_url = os.environ.get("OLLAMA_BASE_URL", "http://127.0.0.1:11434").rstrip("/")
        async with httpx.AsyncClient(timeout=15.0) as c:
            tags = await c.get(f"{ollama_url}/api/tags")
            if tags.status_code != 200:
                pytest.fail(
                    "OLLAMA_CHAT_INTEGRATION=1 requires `ollama serve` on OLLAMA_BASE_URL"
                )

        olla = next((a for a in agents_reg if a.provider_key == "ollama"), None)
        assert olla is not None
        entry = AgentEntry(
            name=olla.name,
            provider="ollama",
            model=olla.model,
            agent_id=olla.agent_id,
            api_key=olla.api_key,
        )
        llm_agent = create_agent(olla.agent_id, sim_id, entry, _sim_base())

        async with httpx.AsyncClient(timeout=15.0) as client:
            clock_r = await client.get(f"{_sim_base()}/api/v1/simulations/{sim_id}")
            clock_r.raise_for_status()
            body = clock_r.json()

        date_str = ""
        if body.get("as_of_date"):
            date_str = str(body["as_of_date"])[:10]
        elif body.get("start_date"):
            date_str = str(body["start_date"])[:10]
        else:
            pytest.fail("cannot resolve simulation calendar date")

        executed = await llm_agent.run_turn(date_str)
        assert isinstance(executed, list)


@pytest.mark.asyncio
async def test_step15_simulation_reaches_completed_and_leaderboard_ok() -> None:
    """Drive ticks until simulator reports ``completed``, then leaderboard is JSON-safe."""
    _require_integration_env()

    redis_url = _redis_url()
    start = os.environ.get("INTEGRATION_START_DATE", "2024-01-02").strip()
    end = os.environ.get("INTEGRATION_END_DATE", "2024-01-05").strip()

    sim_id, _ = await bootstrap_three_agent_simulation(
        base_url=_sim_base(),
        redis_url=redis_url,
        start_date=start,
        end_date=end,
    )

    async with httpx.AsyncClient(timeout=60.0) as client:
        for _ in range(400):
            g = await client.get(f"{_sim_base()}/api/v1/simulations/{sim_id}")
            g.raise_for_status()
            if g.json().get("status") == "completed":
                break

            tick = await client.post(f"{_sim_base()}/api/v1/simulations/{sim_id}/tick")
            if tick.status_code != 200:
                pytest.fail(f"tick failed: {tick.status_code} {tick.text}")

        final = await client.get(f"{_sim_base()}/api/v1/simulations/{sim_id}")
        final.raise_for_status()
        assert final.json().get("status") == "completed"

        lb = await client.get(f"{_sim_base()}/api/v1/leaderboard/{sim_id}")
        lb.raise_for_status()
        rows = lb.json().get("entries", [])
        assert isinstance(rows, list)
