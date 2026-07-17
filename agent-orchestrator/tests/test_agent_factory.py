from agents.factory import create_agent
from agents.team_agent import TradingTeamAgent
from app.config_loader import AgentEntry


def test_create_custom_agent():
    entry = AgentEntry(
        name="rules",
        provider="custom",
        model="momentum-v1",
        agent_id="00000000-0000-4000-8000-000000000001",
        api_key="secret",
    )
    agent = create_agent(entry.agent_id, "sim-id", entry, "http://localhost:8070")
    assert agent.__class__.__name__ == "CustomAgent"


def test_create_gpt_agent_with_custom_prompt_uses_team_by_default():
    entry = {
        "name": "gpt",
        "provider": "gpt",
        "model": "gpt-4.1-nano",
        "agent_id": "00000000-0000-4000-8000-000000000002",
        "api_key": "secret",
        "openai_api_key": "sk-test",
        "system_prompt": "Trade aggressively.",
    }
    agent = create_agent(entry["agent_id"], "sim-id", entry, "http://localhost:8070")
    assert isinstance(agent, TradingTeamAgent)
    assert agent.system_prompt == "Trade aggressively."


def test_create_gpt_agent_single_mode():
    entry = {
        "name": "gpt",
        "provider": "gpt",
        "model": "gpt-4.1-nano",
        "agent_id": "00000000-0000-4000-8000-000000000002",
        "api_key": "secret",
        "openai_api_key": "sk-test",
        "system_prompt": "Trade aggressively.",
        "team_mode": False,
    }
    agent = create_agent(entry["agent_id"], "sim-id", entry, "http://localhost:8070")
    assert agent.__class__.__name__ == "GPTAgent"
    assert agent.system_prompt == "Trade aggressively."
