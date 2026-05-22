from market_client.models import Order


def test_order_defaults():
    o = Order(symbol="RELIANCE.NS", side="buy", quantity=10)
    assert o.order_type == "market"
    assert o.symbol == "RELIANCE.NS"
