# Market simulator (Go)

REST API and simulation engine for virtual order matching, portfolio state, and time advancement. See `docs/rroadmap.md` and `docs/rules.md`.

## Run locally

From **repository root**:

```bash
make dev-simulator
```

Or from this directory:

```bash
go run ./cmd/server
```

Default listen address: `:8070`. Override with `LISTEN_ADDR`.

Build binary (output in `bin/server`, gitignored):

```bash
make go-build    # from repo root
```

## Docker

Built by `infra/docker-compose.yml` as service `market-simulator`. From repo root: `make up`.
