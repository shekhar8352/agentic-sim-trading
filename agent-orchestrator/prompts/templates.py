from __future__ import annotations

import json
from typing import Any


def build_trading_context(
    current_date: str,
    portfolio: dict[str, Any],
    market_data: dict[str, list[dict]],
) -> str:
    """Format portfolio and OHLCV snapshots for LLM context (Step 13)."""
    return f"""Date: {current_date}

Portfolio:
{json.dumps(portfolio, indent=2)}

Market data (OHLCV by symbol):
{json.dumps(market_data, indent=2)}
"""
