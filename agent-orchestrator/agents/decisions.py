"""Parse LLM JSON output into validated Order models (Step 13)."""

from __future__ import annotations

import json
import re
from typing import Any

from pydantic import ValidationError

from market_client.models import Order

_FENCE_RE = re.compile(r"^```(?:json)?\s*(.*?)\s*```$", re.DOTALL | re.IGNORECASE)

# Legacy / mistaken catalog ids → current OpenAI API model names.
GPT_MODEL_ALIASES: dict[str, str] = {
    "gpt-5.1-nano": "gpt-5-nano",
}


def resolve_gpt_model(model: str) -> str:
    return GPT_MODEL_ALIASES.get(model.strip(), model.strip())


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


def _normalize_order_item(item: dict[str, Any]) -> dict[str, Any]:
    out = dict(item)
    if "side" in out and out["side"] is not None:
        out["side"] = str(out["side"]).lower()
    if "order_type" in out and out["order_type"] is not None:
        out["order_type"] = str(out["order_type"]).lower()
    if "symbol" in out and out["symbol"] is not None:
        out["symbol"] = str(out["symbol"]).strip()
    if "quantity" in out and out["quantity"] is not None:
        try:
            out["quantity"] = int(float(out["quantity"]))
        except (TypeError, ValueError):
            pass
    return out


def is_valid_llm_output(raw: str, orders: list[Order]) -> bool:
    """True when the model returned parseable JSON (including an explicit hold [])."""
    if orders:
        return True
    text = extract_json_text(raw.strip())
    if not text:
        return False
    try:
        data: Any = json.loads(text)
    except json.JSONDecodeError:
        return False
    if isinstance(data, list):
        return True
    if isinstance(data, dict):
        action = str(data.get("action", "")).lower()
        if action == "hold" or not data.get("symbol"):
            return True
        return bool(data.get("symbol"))
    return False


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
            orders.append(Order.model_validate(_normalize_order_item(item)))
        except ValidationError:
            continue
    return orders
