"""Request/response models for market_client (roadmap Step 12)."""

from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, ConfigDict, Field, field_validator


class Order(BaseModel):
    symbol: str
    side: Literal["buy", "sell"]
    order_type: Literal["market", "limit"] = "market"
    quantity: int = Field(gt=0)
    price: float | None = None

    @field_validator("symbol")
    @classmethod
    def strip_symbol(cls, value: str) -> str:
        return value.strip()


class HoldingDetail(BaseModel):
    model_config = ConfigDict(extra="allow")

    symbol: str
    quantity: int
    avg_buy_price: float
    mark_price: float = 0.0
    position_value: float = 0.0
    unrealized_pnl: float = 0.0


class PortfolioDetail(BaseModel):
    model_config = ConfigDict(extra="allow")

    simulation_id: str
    agent_id: str
    as_of_date: str
    cash: float
    holdings: list[HoldingDetail] = Field(default_factory=list)
    invested_value: float = 0.0
    total_value: float = 0.0
    total_pnl: float = 0.0
    total_return_pct: float = 0.0
    starting_capital: float = 0.0


class Quote(BaseModel):
    model_config = ConfigDict(extra="allow")

    symbol: str
    date: str | None = None
    open: float = 0.0
    high: float = 0.0
    low: float = 0.0
    close: float = 0.0
    volume: int = 0


class OHLCVBar(BaseModel):
    model_config = ConfigDict(extra="allow")

    symbol: str
    date: str
    open: float
    high: float
    low: float
    close: float
    volume: int = 0


class OrderRecord(BaseModel):
    model_config = ConfigDict(extra="allow")

    id: str
    symbol: str
    order_type: str
    side: str
    quantity: int
    status: str
