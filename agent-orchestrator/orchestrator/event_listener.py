from __future__ import annotations

import asyncio
import json
import logging
from collections.abc import AsyncIterator, Callable
from typing import Any

import redis.asyncio as redis

logger = logging.getLogger(__name__)


class SimulationEventListener:
    """Subscribe to market-simulator JSON events on Redis (default channel ``sim.events``)."""

    def __init__(self, redis_url: str, channel: str = "sim.events"):
        self.redis_url = redis_url
        self.channel = channel

    async def listen(self, handler: Callable[[dict[str, Any]], None]) -> None:
        client = redis.from_url(self.redis_url, decode_responses=True)
        pubsub = client.pubsub()
        await pubsub.subscribe(self.channel)
        logger.info("listening on redis channel=%s", self.channel)
        try:
            async for message in pubsub.listen():
                if message["type"] != "message":
                    continue
                data = json.loads(message["data"])
                handler(data)
        finally:
            await pubsub.unsubscribe(self.channel)
            await client.aclose()

    async def stream(self) -> AsyncIterator[dict[str, Any]]:
        q: asyncio.Queue[dict[str, Any]] = asyncio.Queue()

        def _put(event: dict[str, Any]) -> None:
            q.put_nowait(event)

        task = asyncio.create_task(self.listen(_put))
        try:
            while True:
                yield await q.get()
        finally:
            task.cancel()
