from __future__ import annotations

from functools import lru_cache

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="ignore")

    market_simulator_url: str = "http://localhost:8070"
    redis_url: str | None = None
    database_url: str | None = None

    anthropic_api_key: str | None = None
    openai_api_key: str | None = None
    google_api_key: str | None = None

    agents_config_path: str = "config/agents.yaml"

    # Local Ollama (Phase 4 / Step 15 testing)
    ollama_base_url: str = "http://127.0.0.1:11434"
    ollama_model: str = "gemma3:270m"

    # Step 16 — resilience tuning
    redis_reconnect_base_seconds: float = 1.0
    redis_reconnect_max_delay_seconds: float = 60.0


@lru_cache
def get_settings() -> Settings:
    return Settings()
