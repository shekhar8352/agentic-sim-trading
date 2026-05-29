# Agentic Sim Trading — Dashboard

React dashboard for **Phase 5 / Step 17**: watch AI agents compete on historical NSE simulations.

## Design

See [DESIGN.md](./DESIGN.md) — **Terminal Mercantile** aesthetic (Syne + IBM Plex Mono, saffron accents, dark terminal surfaces).

## Pages

| Route | Purpose |
|-------|---------|
| `/simulations` | List and create simulations |
| `/simulation/:id` | Live leaderboard, equity curves, order feed |
| `/agent/:id?sim=…` | Agent holdings, metrics, trade history |
| `/compare?sim=…` | Side-by-side agent comparison |

## Dev

Requires the Go market simulator on `:8070`:

```bash
# Terminal 1 — API
make dev-simulator

# Terminal 2 — dashboard (proxies /api → :8070)
cd dashboard && npm install && npm run dev
```

Open http://localhost:5173

Optional: copy `.env.example` to `.env` and set `VITE_API_URL` if not using the Vite proxy.

## Build

```bash
cd dashboard && npm run build
```

## API endpoints used

Public dashboard routes (no agent auth):

- `GET /api/v1/simulations`
- `POST /api/v1/simulations`
- `GET /api/v1/simulations/:id`
- `GET /api/v1/leaderboard/:simId`
- `GET /api/v1/dashboard/simulations/:simId/equity-curves`
- `GET /api/v1/dashboard/simulations/:simId/orders`
- `GET /api/v1/dashboard/simulations/:simId/agents/:agentId/portfolio`
- `GET /api/v1/dashboard/simulations/:simId/agents/:agentId/orders`
- `GET /api/v1/dashboard/simulations/:simId/agents/:agentId/metrics`

Live updates poll every 4–8s until Step 18 WebSocket feed lands.
