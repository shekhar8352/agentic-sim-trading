from __future__ import annotations

import logging
from typing import Any

from agents.base_agent import BaseAgent

logger = logging.getLogger(__name__)


class AgentRunner:
    """Main agent loop controller (roadmap Step 11 scaffold; wired in Step 14+)."""

    def __init__(self, agents: list[BaseAgent]):
        self.agents = agents

    async def run_turn(self, current_date: str) -> list[dict[str, Any]]:
        results: list[dict[str, Any]] = []
        for agent in self.agents:
            logger.info("agent turn agent_id=%s date=%s", agent.agent_id, current_date)
            decision = await agent.act(current_date)
            results.append({"agent_id": agent.agent_id, "decision": decision})
        return results
