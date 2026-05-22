"""Parse LLM JSON output into validated Order models (Step 13)."""

from __future__ import annotations

import json
import re
from typing import Any

from pydantic import ValidationError

from market_client.models import Order

_FENCE_RE = re.compile(r"^```(?:json)?\s*(.*?)\s*```$", re.DOTALL | re.IGNORECASE)


def extract_json_text(raw: str) -> str:
    text = raw.strip()
    match = _FENCE_RE.match(text)
    if match:
        return match.group(1).strip()
    start = text.find("[")
    end = text.rfind("]")
    if start != -1 and end > start:
        return text[start : end + 1]
    start = text.find("{")
    end = text.rfind("}")
    if start != -1 and end > start:
        return text[start : end + 1]
    return text


def parse_orders_json(raw: str) -> list[Order]:
    """Parse a JSON array (or single object / hold) into Order instances."""
    try:
        data: Any = json.loads(extract_json_text(raw))
    except json.JSONDecodeError:
        return []

    if isinstance(data, dict):
        action = str(data.get("action", "")).lower()
        if action == "hold" or not data.get("symbol"):
            return []
        data = [data]

    if not isinstance(data, list):
        return []

    orders: list[Order] = []
    for item in data[:10]:
        if not isinstance(item, dict):
            continue
        try:
            orders.append(Order.model_validate(item))
        except ValidationError:
            continue
    return orders
