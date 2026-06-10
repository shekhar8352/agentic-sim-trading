from fastapi.testclient import TestClient

from app.main import app


def test_providers_endpoint():
    client = TestClient(app)
    resp = client.get("/api/v1/providers")
    assert resp.status_code == 200
    body = resp.json()
    assert "providers" in body
    assert "default_system_prompt" in body
    assert "personalities" in body
    assert "strategies" in body
    assert any(p["id"] == "custom" for p in body["providers"])
    assert any(p["id"] == "risk_taker" for p in body["personalities"])


def test_personality_prompt_endpoint():
    client = TestClient(app)
    resp = client.get("/api/v1/personalities/risk_taker/prompt")
    assert resp.status_code == 200
    body = resp.json()
    assert body["personality"] == "risk_taker"
    assert "risk-taking" in body["system_prompt"].lower()
