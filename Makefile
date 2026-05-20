.DEFAULT_GOAL := help

.PHONY: help up down build ps logs \
	dev-simulator dev-orchestrator install-orchestrator install-orchestrator-ingest ingest-data \
	go-build go-test go-mod-tidy lint-orchestrator

COMPOSE := docker compose -f infra/docker-compose.yml

help:
	@echo "Agentic sim trading — common commands"
	@echo ""
	@echo "  make up              Start full stack (Docker Compose, build if needed)"
	@echo "  make down            Stop and remove Compose containers"
	@echo "  make build           Build Compose images only"
	@echo "  make ps              Compose service status"
	@echo "  make logs            Follow Compose logs (all services)"
	@echo ""
	@echo "  make dev-simulator   Run Go market-simulator locally (:8070)"
	@echo "  make dev-orchestrator  Run FastAPI orchestrator locally (:8071; install first)"
	@echo "  make install-orchestrator  pip install -e '.[dev]' in agent-orchestrator/"
	@echo "  make install-orchestrator-ingest  pip install '.[dev,ingest]' (yfinance + DB load)"
	@echo "  make ingest-data         Run scripts/ingest_data.py (Postgres must be up)"
	@echo ""
	@echo "  make go-build        go build market-simulator"
	@echo "  make go-test         go test ./... in market-simulator"
	@echo "  make go-mod-tidy     go mod tidy in market-simulator"
	@echo "  make lint-orchestrator   ruff check agent-orchestrator (after install)"

up:
	$(COMPOSE) up --build

down:
	$(COMPOSE) down

build:
	$(COMPOSE) build

ps:
	$(COMPOSE) ps

logs:
	$(COMPOSE) logs -f

dev-simulator:
	cd market-simulator && go run ./cmd/server

install-orchestrator:
	cd agent-orchestrator && pip install -e ".[dev]"

install-orchestrator-ingest:
	cd agent-orchestrator && pip install -e ".[dev,ingest]"

ingest-data:
	cd agent-orchestrator && python scripts/ingest_data.py

dev-orchestrator:
	cd agent-orchestrator && uvicorn app.main:app --reload --host 0.0.0.0 --port 8071

go-build:
	cd market-simulator && go build -o bin/server ./cmd/server

go-test:
	cd market-simulator && go test ./...

go-mod-tidy:
	cd market-simulator && go mod tidy

lint-orchestrator:
	cd agent-orchestrator && ruff check app orchestrator agents market_client prompts analytics tests
