from __future__ import annotations

from collections import deque

from agents.base_agent import BaseAgent


class AgentScheduler:
    """Round-robin scheduling for which agent acts next (Step 14+)."""

    def __init__(self, agents: list[BaseAgent]):
        self._queue: deque[BaseAgent] = deque(agents)

    def next_agent(self) -> BaseAgent | None:
        if not self._queue:
            return None
        agent = self._queue.popleft()
        self._queue.append(agent)
        return agent
