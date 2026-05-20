FROM python:3.12-slim
WORKDIR /app

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
ENV MARKET_SIMULATOR_URL=http://market-simulator:8070

COPY pyproject.toml README.md ./
COPY app ./app
COPY orchestrator ./orchestrator
COPY agents ./agents
COPY market_client ./market_client
COPY prompts ./prompts
COPY analytics ./analytics
COPY config ./config

RUN pip install --no-cache-dir -U pip && pip install --no-cache-dir .

EXPOSE 8071
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8071"]
