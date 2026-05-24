from __future__ import annotations

from dataclasses import dataclass

import httpx

from app.settings import Settings
from prompts.system import TRADING_SYSTEM_PROMPT


@dataclass(frozen=True)
class ProviderSpec:
    id: str
    label: str
    models: tuple[str, ...]
    requires_api_key: bool = True


DEFAULT_OLLAMA_MODELS: tuple[str, ...] = ("gemma3:270m", "gemma4:latest", "llama3.2:latest")

PROVIDER_CATALOG: tuple[ProviderSpec, ...] = (
    ProviderSpec(
        "gpt",
        "OpenAI",
        (
            "gpt-5.4-nano",
            "gpt-5-nano",
            "gpt-4.1-nano",
            "gpt-4.1-mini",
            "gpt-4o",
            "gpt-4o-mini",
        ),
    ),
    ProviderSpec("claude", "Anthropic", ("claude-sonnet-4-20250514", "claude-3-5-haiku-20241022")),
    ProviderSpec("gemini", "Google Gemini", ("gemini-2.0-flash", "gemini-1.5-pro")),
    ProviderSpec("ollama", "Ollama (local)", DEFAULT_OLLAMA_MODELS, requires_api_key=False),
    ProviderSpec("custom", "Rules (no LLM)", ("momentum-v1",), requires_api_key=False),
)


def _ollama_reachable(settings: Settings) -> bool:
    base = (settings.ollama_base_url or "").rstrip("/")
    if not base:
        return False
    try:
        with httpx.Client(timeout=2.0) as client:
            response = client.get(f"{base}/api/tags")
            return response.status_code == 200
    except Exception:
        return False


def _ollama_models(settings: Settings) -> list[str]:
    base = (settings.ollama_base_url or "").rstrip("/")
    remote: list[str] = []
    if base:
        try:
            with httpx.Client(timeout=2.0) as client:
                response = client.get(f"{base}/api/tags")
                if response.status_code == 200:
                    remote = [
                        str(item.get("name"))
                        for item in response.json().get("models", [])
                        if item.get("name")
                    ]
        except Exception:
            pass

    merged: list[str] = []
    seen: set[str] = set()
    preferred = settings.ollama_model.strip() if settings.ollama_model else ""
    for name in [preferred, *remote, *DEFAULT_OLLAMA_MODELS]:
        if not name or name in seen:
            continue
        merged.append(name)
        seen.add(name)
    return merged


def _provider_available(spec: ProviderSpec, settings: Settings) -> bool:
    if not spec.requires_api_key:
        if spec.id == "ollama":
            return _ollama_reachable(settings)
        return True
    if spec.id in ("gpt", "openai"):
        return bool(settings.openai_api_key)
    if spec.id == "claude":
        return bool(settings.anthropic_api_key)
    if spec.id == "gemini":
        return bool(settings.google_api_key)
    return False


def list_providers(settings: Settings) -> list[dict]:
    out: list[dict] = []
    for spec in PROVIDER_CATALOG:
        models = _ollama_models(settings) if spec.id == "ollama" else list(spec.models)
        out.append(
            {
                "id": spec.id,
                "label": spec.label,
                "available": _provider_available(spec, settings),
                "models": models,
                "requires_api_key": spec.requires_api_key,
            }
        )
    return out


def default_system_prompt() -> str:
    return TRADING_SYSTEM_PROMPT
