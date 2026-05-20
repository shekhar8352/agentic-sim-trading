from dotenv import load_dotenv
from fastapi import FastAPI

from app.config_loader import load_agents_config
from app.settings import get_settings

load_dotenv()

app = FastAPI(title="Agent orchestrator", version="0.1.0")


@app.get("/health")
def health():
    settings = get_settings()
    cfg = load_agents_config(settings.agents_config_path)
    return {
        "status": "ok",
        "service": "agent-orchestrator",
        "market_simulator_url": settings.market_simulator_url,
        "redis_configured": bool(settings.redis_url),
        "agents_configured": len(cfg.agents),
    }
