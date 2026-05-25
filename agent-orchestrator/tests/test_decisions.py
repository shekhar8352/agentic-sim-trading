from agents.decisions import (
    extract_json_text,
    is_valid_llm_output,
    parse_orders_json,
    resolve_gpt_model,
)


def test_parse_orders_array():
    raw = '[{"symbol":"TCS.NS","side":"buy","order_type":"market","quantity":3}]'
    orders = parse_orders_json(raw)
    assert len(orders) == 1
    assert orders[0].symbol == "TCS.NS"
    assert orders[0].quantity == 3


def test_parse_orders_from_fence():
    raw = """```json
[{"symbol":"INFY.NS","side":"sell","order_type":"market","quantity":2}]
```"""
    orders = parse_orders_json(raw)
    assert len(orders) == 1
    assert orders[0].side == "sell"


def test_parse_hold_object():
    orders = parse_orders_json('{"action":"hold","symbol":"","quantity":0}')
    assert orders == []


def test_parse_invalid_returns_empty():
    assert parse_orders_json("not json") == []


def test_parse_hold_empty_array():
    assert parse_orders_json("[]") == []
    assert is_valid_llm_output("[]", []) is True


def test_parse_orders_normalizes_side_case():
    raw = '[{"symbol":"TCS.NS","side":"BUY","order_type":"MARKET","quantity":3}]'
    orders = parse_orders_json(raw)
    assert len(orders) == 1
    assert orders[0].side == "buy"


def test_resolve_gpt_model_alias():
    assert resolve_gpt_model("gpt-5.1-nano") == "gpt-5-nano"


def test_extract_json_text_finds_array():
    text = extract_json_text('Here you go:\n[{"symbol":"X.NS","side":"buy","quantity":1}]')
    assert text.startswith("[")
