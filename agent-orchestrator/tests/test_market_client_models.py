from market_client.client import Order


def test_order_model():
    o = Order(symbol="RELIANCE.NS", side="buy", quantity=10)
    assert o.order_type == "market"
