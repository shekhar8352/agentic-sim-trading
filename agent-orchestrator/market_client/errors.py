"""HTTP client errors for the Go market-simulator API."""

from __future__ import annotations


class MarketClientError(Exception):
    """Base error for market_client failures."""


class APIError(MarketClientError):
    """Non-success HTTP response from the simulator."""

    def __init__(self, status_code: int, detail: str):
        self.status_code = status_code
        self.detail = detail
        super().__init__(f"HTTP {status_code}: {detail}")


class ConfigurationError(MarketClientError):
    """Missing required client configuration (e.g. simulation_id)."""


class TransportError(MarketClientError):
    """Network or timeout failure talking to the simulator."""

    def __init__(self, message: str, *, cause: BaseException | None = None):
        self.cause = cause
        super().__init__(message)
