from unittest.mock import MagicMock, patch

from app.providers import list_providers
from app.settings import Settings


def test_list_providers_marks_openai_when_key_set():
    settings = Settings(openai_api_key="sk-test")
    providers = {p["id"]: p for p in list_providers(settings)}
    assert providers["gpt"]["available"] is True
    assert providers["custom"]["available"] is True
    assert providers["claude"]["available"] is False


def test_default_models_for_gpt():
    settings = Settings(openai_api_key="sk-test")
    gpt = next(p for p in list_providers(settings) if p["id"] == "gpt")
    assert "gpt-5-nano" in gpt["models"]
    assert "gpt-5.4-nano" in gpt["models"]
    assert "gpt-4.1-nano" in gpt["models"]


@patch("app.providers.httpx.Client")
def test_ollama_models_include_pulled_tags(mock_client_cls):
    response = MagicMock()
    response.status_code = 200
    response.json.return_value = {"models": [{"name": "gemma3:270m"}, {"name": "gemma4:latest"}]}
    client = MagicMock()
    client.__enter__.return_value = client
    client.get.return_value = response
    mock_client_cls.return_value = client

    settings = Settings(ollama_model="gemma3:270m")
    ollama = next(p for p in list_providers(settings) if p["id"] == "ollama")
    assert ollama["available"] is True
    assert ollama["models"][0] == "gemma3:270m"
    assert "gemma4:latest" in ollama["models"]
