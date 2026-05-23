"""Run the Redis-backed orchestrator: ``python -m orchestrator`` (from ``agent-orchestrator/``)."""

from __future__ import annotations

import asyncio
import logging
import sys

from dotenv import load_dotenv


def main() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(levelname)s %(name)s %(message)s",
    )
    load_dotenv()
    try:
        asyncio.run(_async_main())
    except KeyboardInterrupt:
        logging.info("interrupt — exiting")


async def _async_main() -> None:
    from agents.factory import create_agent
    from app.config_loader import load_agents_config
    from app.settings import get_settings
    from orchestrator.runner import SimulationRunner

    settings = get_settings()
    if not settings.redis_url:
        print("REDIS_URL is required to subscribe to simulation events.", file=sys.stderr)
        sys.exit(1)

    cfg = load_agents_config(settings.agents_config_path)
    if not cfg.simulation_id:
        print(
            "Set simulation_id in agents.yaml to the UUID of the running simulation.",
            file=sys.stderr,
        )
        sys.exit(1)

    go_base = cfg.go_service_url or settings.market_simulator_url
    agents = []
    for entry in cfg.agents:
        try:
            agents.append(
                create_agent(
                    entry.agent_id or "",
                    cfg.simulation_id,
                    entry,
                    go_base,
                )
            )
        except ValueError as exc:
            logging.warning("skip agent %s: %s", entry.name, exc)

    if not agents:
        print(
            "No agents loaded. Each agents.yaml entry needs agent_id and api_key.",
            file=sys.stderr,
        )
        sys.exit(1)

    runner = SimulationRunner(
        agents,
        settings.redis_url,
        simulation_id=cfg.simulation_id,
    )
    logging.info(
        "starting orchestrator simulation_id=%s market_simulator=%s agents=%d",
        cfg.simulation_id,
        go_base,
        len(agents),
    )
    await runner.start()


if __name__ == "__main__":
    main()
