from fastapi.testclient import TestClient

from app.main import app


def test_providers_endpoint():
    client = TestClient(app)
    resp = client.get("/api/v1/providers")
    assert resp.status_code == 200
    body = resp.json()
    assert "providers" in body
    assert "default_system_prompt" in body
    assert any(p["id"] == "custom" for p in body["providers"])
