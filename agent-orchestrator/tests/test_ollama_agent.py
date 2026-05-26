from __future__ import annotations

import httpx
import pytest

from agents.ollama_agent import OllamaAgent


@pytest.mark.asyncio
async def test_ollama_complete_extracts_chat_message(monkeypatch: pytest.MonkeyPatch):
    captured: dict[str, object] = {}

    class FakeResponse:
        def raise_for_status(self) -> None:
            return None

        def json(self):
            return {
                "message": {
                    "role": "assistant",
                    "content": (
                        '[{"symbol":"TCS.NS","side":"buy","order_type":"market","quantity":1}]'
                    ),
                }
            }

    class FakeAsyncClient:
        def __init__(self, *args: object, timeout: float = 0, **kwargs: object) -> None:
            pass

        async def __aenter__(self):
            return self

        async def __aexit__(self, *exc: object) -> None:
            return None

        async def post(self, url, json=None):
            captured["url"] = url
            captured["json"] = json
            return FakeResponse()

    monkeypatch.setattr(httpx, "AsyncClient", FakeAsyncClient)

    agent = OllamaAgent(
        "00000000-0000-4000-a000-000000000099",
        "sim-zz",
        {
            "go_service_url": "http://localhost:8070",
            "api_key": "k",
            "model": "gemma4:latest",
            "ollama_base_url": "http://ollama.local:11434",
        },
    )

    raw = await agent.complete("hello")
    assert "TCS.NS" in raw
    assert captured["url"] == "http://ollama.local:11434/api/chat"
    payload = captured["json"]
    assert isinstance(payload, dict)
    assert payload["model"] == "gemma4:latest"
    assert payload["stream"] is False


def test_create_ollama_agent_via_factory():
    from agents.factory import create_agent
    from app.config_loader import AgentEntry

    entry = AgentEntry(
        name="locallm",
        provider="ollama",
        model="gemma4:latest",
        agent_id="00000000-0000-4000-a000-000000000098",
        api_key="sim-key",
        ollama_base_url="http://127.0.0.1:11434",
    )
    a = create_agent(entry.agent_id, "sim-local", entry, "http://localhost:8070")
    assert isinstance(a, OllamaAgent)
