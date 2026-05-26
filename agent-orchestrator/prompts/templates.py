from __future__ import annotations

from typing import Any


def _fmt_holdings(holdings: list[dict]) -> str:
    if not holdings:
        return "  (none)"
    lines = []
    for h in holdings:
        sym = h.get("symbol", "?")
        qty = h.get("quantity", 0)
        avg = float(h.get("avg_buy_price", h.get("avg_cost", 0)))
        cur = float(h.get("mark_price", h.get("current_price", avg)))
        pnl = (cur - avg) * qty if avg else 0
        pnl_pct = ((cur - avg) / avg * 100) if avg else 0
        lines.append(
            f"  {sym}: qty={qty}  avg=₹{avg:.2f}  last=₹{cur:.2f}"
            f"  pnl=₹{pnl:+,.2f} ({pnl_pct:+.1f}%)"
        )
    return "\n".join(lines)


def _fmt_ohlcv(market_data: dict[str, list[dict]]) -> str:
    """Compact OHLCV table: one row per day, saves ~60% tokens vs json.dumps."""
    blocks: list[str] = []
    for symbol, candles in market_data.items():
        if not candles:
            continue
        header = f"{symbol} (O  H  L  C  Vol)"
        rows = []
        for c in candles[-20:]:
            date = str(c.get("date", c.get("timestamp", "")))[:10]
            o = float(c.get("open", 0))
            h = float(c.get("high", 0))
            lv = float(c.get("low", 0))
            cl = float(c.get("close", 0))
            vol = int(c.get("volume", 0))
            rows.append(f"  {date}  {o:.2f}  {h:.2f}  {lv:.2f}  {cl:.2f}  {vol:,}")
        blocks.append(header + "\n" + "\n".join(rows))
    return "\n\n".join(blocks)


def build_trading_context(
    current_date: str,
    portfolio: dict[str, Any],
    market_data: dict[str, list[dict]],
) -> str:
    """Format portfolio and OHLCV snapshots for LLM context."""
    cash = float(portfolio.get("cash", 0))
    total_value = float(portfolio.get("total_value", cash))
    pnl_pct = float(portfolio.get("total_return_pct", portfolio.get("pnl_pct", 0)))
    holdings: list[dict] = portfolio.get("holdings", [])

    return (
        f"=== TRADING TURN: {current_date} ===\n"
        f"\nPORTFOLIO\n"
        f"  Cash available : ₹{cash:>14,.2f}\n"
        f"  Total value    : ₹{total_value:>14,.2f}\n"
        f"  Overall P&L    : {pnl_pct:+.2f}%\n"
        f"\nHOLDINGS\n"
        f"{_fmt_holdings(holdings)}\n"
        f"\nMARKET DATA (last 20 trading days)\n"
        f"{_fmt_ohlcv(market_data)}\n"
        f"\nOutput ONLY a JSON array of orders, e.g.:\n"
        f'[{{"symbol":"RELIANCE.NS","side":"buy","order_type":"market","quantity":10}}]\n'
        f"Return [] to hold.\n"
    )
