from agents.factory import create_agent
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
