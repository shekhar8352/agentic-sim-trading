from contextlib import asynccontextmanager

from dotenv import load_dotenv
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.config_loader import load_agents_config
from app.routes import router
from app.settings import get_settings
from orchestrator.manager import get_manager

load_dotenv()


@asynccontextmanager
async def lifespan(app: FastAPI):
    yield
    await get_manager().stop_all()


app = FastAPI(title="Agent orchestrator", version="0.1.0", lifespan=lifespan)
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)
app.include_router(router)


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
        "openai_configured": bool(settings.openai_api_key),
        "anthropic_configured": bool(settings.anthropic_api_key),
        "google_configured": bool(settings.google_api_key),
    }
