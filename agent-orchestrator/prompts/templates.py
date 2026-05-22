from __future__ import annotations

import json
from typing import Any


def build_trading_context(
    current_date: str,
    portfolio: dict[str, Any],
    market_data: dict[str, list[dict]],
) -> str:
    """Format portfolio and OHLCV snapshots for LLM context (Step 13)."""
    cash = float(portfolio.get("cash", 0))
    total_value = float(portfolio.get("total_value", cash))
    pnl_pct = float(portfolio.get("total_return_pct", portfolio.get("pnl_pct", 0)))
    holdings = portfolio.get("holdings", [])

    return f"""Date: {current_date}

Your Portfolio:
- Cash: ₹{cash:,.2f}
- Holdings: {json.dumps(holdings, indent=2)}
- Total Value: ₹{total_value:,.2f}
- P&L: {pnl_pct:.2f}%

Market Data (last 20 days OHLCV):
{json.dumps(market_data, indent=2)}

Rules:
- Max 20% of capital in any single stock
- Only BUY and SELL market orders
- You can place up to 10 orders this turn
- Return ONLY a JSON array of orders
"""
