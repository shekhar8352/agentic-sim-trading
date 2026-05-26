from __future__ import annotations

from orchestrator.resilience import compute_backoff_delay


def test_compute_backoff_delay_exponential():
    assert compute_backoff_delay(1, base_seconds=1.0) == 1.0
    assert compute_backoff_delay(2, base_seconds=1.0) == 2.0
    assert compute_backoff_delay(3, base_seconds=1.0) == 4.0


def test_compute_backoff_delay_respects_max():
    assert compute_backoff_delay(20, base_seconds=1.0, max_delay_seconds=30.0) == 30.0
