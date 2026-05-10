# Agent orchestrator (Python / FastAPI)

Runs AI agents, manages prompts, and talks to the Go **market-simulator** service. See `docs/rroadmap.md` and `docs/rules.md`.

## Run locally

```bash
cd agent-orchestrator
python -m venv .venv
source .venv/bin/activate  # Windows: .venv\Scripts\activate
pip install -e .
uvicorn app.main:app --reload --host 0.0.0.0 --port 8071
```

Set `MARKET_SIMULATOR_URL` if the simulator is not on `http://localhost:8070`.

## Docker

Built by `infra/docker-compose.yml` as service `agent-orchestrator`.
