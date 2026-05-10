# Market simulator (Go)

REST API and simulation engine for virtual order matching, portfolio state, and time advancement. See `docs/rroadmap.md` and `docs/rules.md`.

## Run locally

```bash
go run ./cmd/server
```

Default listen address: `:8070`. Override with `LISTEN_ADDR`.

## Docker

Built by `infra/docker-compose.yml` as service `market-simulator`.
