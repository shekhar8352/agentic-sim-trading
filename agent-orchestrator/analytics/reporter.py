from __future__ import annotations

from typing import Any


class AgentReporter:
    """Performance summaries per agent (roadmap Step 11 scaffold; expand in later steps)."""

    def __init__(self, simulation_id: str):
        self.simulation_id = simulation_id

    def summarize(self, leaderboard: list[dict[str, Any]]) -> dict[str, Any]:
        return {
            "simulation_id": self.simulation_id,
            "entry_count": len(leaderboard),
            "entries": leaderboard,
        }
