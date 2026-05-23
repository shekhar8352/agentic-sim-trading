from __future__ import annotations

import asyncio
import json
import logging
from typing import Any

import redis.asyncio as redis

from agents.base_agent import BaseAgent

logger = logging.getLogger(__name__)


class AgentRunner:
    """Sequential controller for invoking agents each turn (testing / simple runs)."""

    def __init__(self, agents: list[BaseAgent]):
        self.agents = agents

    async def run_turn(self, current_date: str) -> list[dict[str, Any]]:
        results: list[dict[str, Any]] = []
        for agent in self.agents:
            logger.info("agent turn agent_id=%s date=%s", agent.agent_id, current_date)
            placed = await agent.run_turn(current_date)
            results.append({"agent_id": agent.agent_id, "orders": placed})
        return results


class SimulationRunner:
    """Redis event loop: react to ``sim.tick`` / ``sim.completed`` from market-simulator (Step 14).

    Go publishes JSON to Redis channel ``sim.events`` with an ``event`` field
    (``sim.tick``, ``sim.completed``, etc.) plus ``simulation_id`` and ``date``.
    """

    def __init__(
        self,
        agents: list[BaseAgent],
        redis_url: str,
        *,
        channel: str = "sim.events",
        simulation_id: str | None = None,
        stop_on_completed: bool = True,
    ):
        self.agents = list(agents)
        self.redis_url = redis_url
        self.channel = channel
        self.simulation_id = simulation_id
        self.stop_on_completed = stop_on_completed
        self._stop_requested = False

    def _matches_sim(self, payload: dict[str, Any]) -> bool:
        if self.simulation_id is None:
            return True
        sid = payload.get("simulation_id")
        return str(sid) == str(self.simulation_id)

    async def handle_tick(self, data: dict[str, Any]) -> None:
        current_date = data.get("date")
        if not current_date:
            logger.warning("sim.tick missing date: %s", data)
            return
        logger.info(
            "tick date=%s simulation_id=%s agents=%d",
            current_date,
            data.get("simulation_id"),
            len(self.agents),
        )
        results = await asyncio.gather(
            *[agent.run_turn(str(current_date)) for agent in self.agents],
            return_exceptions=True,
        )
        for agent, res in zip(self.agents, results):
            if isinstance(res, BaseException):
                logger.error(
                    "agent turn failed agent_id=%s",
                    agent.agent_id,
                    exc_info=res,
                )
            else:
                logger.info(
                    "agent turn done agent_id=%s order_results=%d",
                    agent.agent_id,
                    len(res),
                )

    async def handle_completion(self, data: dict[str, Any]) -> None:
        logger.info(
            "sim.completed simulation_id=%s payload=%s",
            data.get("simulation_id"),
            data,
        )
        if self.stop_on_completed and self._matches_sim(data):
            self._stop_requested = True

    async def dispatch_event(self, payload: dict[str, Any]) -> None:
        """Handle one decoded Redis payload (used by tests and optional callers)."""
        if not self._matches_sim(payload):
            return
        event = payload.get("event")
        if event == "sim.tick":
            await self.handle_tick(payload)
        elif event == "sim.completed":
            await self.handle_completion(payload)
        elif event in ("sim.started", "sim.resumed", "sim.tick.processed"):
            logger.debug("lifecycle/event note: %s", event)
        else:
            logger.debug("unhandled event: %s", event)

    async def start(self) -> None:
        client = redis.from_url(self.redis_url, decode_responses=True)
        pubsub = client.pubsub()
        await pubsub.subscribe(self.channel)
        logger.info(
            "orchestrator listening channel=%s simulation_filter=%s agents=%d",
            self.channel,
            self.simulation_id if self.simulation_id is not None else "(any)",
            len(self.agents),
        )
        try:
            async for message in pubsub.listen():
                if self._stop_requested:
                    break
                if message["type"] != "message":
                    continue
                raw = message.get("data")
                if not isinstance(raw, str):
                    continue
                try:
                    data = json.loads(raw)
                except json.JSONDecodeError:
                    logger.warning("invalid JSON on %s: %r", self.channel, raw[:200])
                    continue
                if not isinstance(data, dict):
                    continue
                await self.dispatch_event(data)
                if self._stop_requested:
                    break
        finally:
            await pubsub.unsubscribe(self.channel)
            await pubsub.aclose()
            await client.aclose()
            logger.info("orchestrator stopped")
