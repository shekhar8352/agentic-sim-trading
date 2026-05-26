"""Backoff helpers for Redis reconnect and other retry loops (Step 16)."""

from __future__ import annotations


def compute_backoff_delay(
    attempt: int,
    *,
    base_seconds: float = 1.0,
    max_delay_seconds: float = 60.0,
) -> float:
    """Exponential backoff capped at ``max_delay_seconds`` (attempt is 1-based)."""
    if attempt < 1:
        attempt = 1
    return min(max_delay_seconds, base_seconds * (2 ** min(attempt - 1, 6)))
