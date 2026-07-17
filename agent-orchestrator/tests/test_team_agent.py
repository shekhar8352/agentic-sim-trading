from __future__ import annotations

from unittest.mock import AsyncMock

import pytest

from agents.decisions import parse_orders_json
from agents.factory import create_agent
from agents.llm_agent import LLMAgent
from agents.team_agent import TradingTeamAgent, wrap_with_team
from market_client.models import Order
from prompts.roles import (
    DEFAULT_TEAM_ROLES,
    normalize_team_roles,
    system_prompt_for_role,
)


class StubLLM(LLMAgent):
    def __init__(self, replies: list[str] | None = None):
        super().__init__(
            "agent-1",
            "sim-1",
            {
                "go_service_url": "http://x",
                "api_key": "k",
                "system_prompt": "Trade carefully.",
            },
        )
        self.replies = list(replies or [])
        self.prompts_seen: list[str] = []
        self.contexts_seen: list[str] = []

    async def complete(self, context: str) -> str:
        self.prompts_seen.append(self.system_prompt)
        self.contexts_seen.append(context)
        if not self.replies:
            return "[]"
        return self.replies.pop(0)


def test_normalize_team_roles_orders_head_last():
    assert normalize_team_roles(["head", "analyst"]) == ("analyst", "head")
    assert normalize_team_roles(None) == DEFAULT_TEAM_ROLES


def test_normalize_team_roles_rejects_unknown():
    with pytest.raises(ValueError, match="unknown team role"):
        normalize_team_roles(["analyst", "intern"])


def test_system_prompt_for_head_includes_personality():
    prompt = system_prompt_for_role("head", head_personality_prompt="Be aggressive.")
    assert "Head Trader" in prompt
    assert "Be aggressive." in prompt


@pytest.mark.asyncio
async def test_team_pipeline_calls_roles_then_parses_head_orders():
    replies = [
        '{"market_bias":"bullish","summary":"up","bullish":[],"bearish":[],"watch":[],"holding_alerts":[]}',
        '{"risk_posture":"balanced","max_new_position_pct":20,"min_cash_pct":10,"summary":"ok","constraints":[],"reduce":[],"block_buys":[]}',
        '{"stance":"accumulate","summary":"buy tcs","ideas":[{"symbol":"TCS.NS","side":"buy","priority":1,"rationale":"momo","sizing_hint":"small"}]}',
        '[{"symbol":"TCS.NS","side":"buy","order_type":"market","quantity":3}]',
    ]
    backend = StubLLM(replies)
    team = TradingTeamAgent(backend)

    orders = await team.decide("=== TRADING TURN ===\ncash ok")
    assert len(orders) == 1
    assert orders[0].symbol == "TCS.NS"
    assert orders[0].quantity == 3
    assert set(team._last_briefing) == {"analyst", "risk_officer", "strategist", "head"}
    assert len(backend.prompts_seen) == 4
    assert "Market Analyst" in backend.prompts_seen[0]
    assert "Risk Officer" in backend.prompts_seen[1]
    assert "Strategist" in backend.prompts_seen[2]
    assert "Head Trader" in backend.prompts_seen[3]
    # Head receives teammate reports in the user payload.
    assert "Analyst" in backend.contexts_seen[3] or "TEAMMATE REPORTS" in backend.contexts_seen[3]


@pytest.mark.asyncio
async def test_team_holds_when_head_output_unparseable():
    backend = StubLLM(
        [
            '{"summary":"a"}',
            '{"summary":"r"}',
            '{"summary":"s"}',
            "not-json-at-all",
        ]
    )
    team = TradingTeamAgent(backend, roles=["analyst", "risk_officer", "strategist", "head"])
    orders = await team.decide("ctx")
    assert orders == []


@pytest.mark.asyncio
async def test_analyst_only_then_head():
    backend = StubLLM(
        [
            '{"market_bias":"neutral","summary":"flat","bullish":[],"bearish":[],"watch":[],"holding_alerts":[]}',
            "[]",
        ]
    )
    team = TradingTeamAgent(backend, roles=["analyst", "head"])
    assert team.roles == ("analyst", "head")
    orders = await team.decide("ctx")
    assert orders == []
    assert list(team._last_briefing.keys()) == ["analyst", "head"]


def test_wrap_with_team_skips_custom():
    from agents.custom_agent import CustomAgent

    custom = CustomAgent("a", "s", {"go_service_url": "http://x", "api_key": "k"})
    assert wrap_with_team(custom, team_mode=True) is custom


def test_factory_defaults_llm_to_team_mode():
    entry = {
        "name": "gpt",
        "provider": "gpt",
        "model": "gpt-4o",
        "agent_id": "00000000-0000-4000-8000-000000000002",
        "api_key": "secret",
        "openai_api_key": "sk-test",
        "system_prompt": "Trade aggressively.",
    }
    agent = create_agent(entry["agent_id"], "sim-id", entry, "http://localhost:8070")
    assert isinstance(agent, TradingTeamAgent)
    assert agent.system_prompt == "Trade aggressively."
    assert agent.roles == DEFAULT_TEAM_ROLES


def test_factory_can_disable_team_mode():
    entry = {
        "name": "gpt",
        "provider": "gpt",
        "model": "gpt-4o",
        "agent_id": "00000000-0000-4000-8000-000000000003",
        "api_key": "secret",
        "openai_api_key": "sk-test",
        "team_mode": False,
        "system_prompt": "Solo trader.",
    }
    agent = create_agent(entry["agent_id"], "sim-id", entry, "http://localhost:8070")
    assert not isinstance(agent, TradingTeamAgent)
    assert agent.system_prompt == "Solo trader."


def test_parse_orders_still_works():
    orders = parse_orders_json('[{"symbol":"INFY.NS","side":"buy","order_type":"market","quantity":1}]')
    assert isinstance(orders[0], Order)


@pytest.mark.asyncio
async def test_act_includes_briefing():
    backend = StubLLM(["{}", "{}", "{}", "[]"])
    team = TradingTeamAgent(backend)
    team.build_context = AsyncMock(return_value="ctx")
    team.client.place_order = AsyncMock()
    result = await team.act("2024-01-02")
    assert result["orders"] == []
    assert "roles" in result
    assert "briefing" in result
