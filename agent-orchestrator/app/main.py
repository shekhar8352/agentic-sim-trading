import os

from fastapi import FastAPI

app = FastAPI(title="Agent orchestrator", version="0.1.0")

MARKET_SIM_URL = os.environ.get("MARKET_SIMULATOR_URL", "http://localhost:8070")


@app.get("/health")
def health():
    return {"status": "ok", "service": "agent-orchestrator", "market_simulator_url": MARKET_SIM_URL}
