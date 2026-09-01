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


def _bar_label(c: dict) -> str:
    ts = str(c.get("ts") or "")
    if ts:
        return ts.replace("T", " ")[:16]
    return str(c.get("date", c.get("timestamp", "")))[:10]


def _fmt_ohlcv(market_data: dict[str, list[dict]], *, hourly: bool = False) -> str:
    """Compact OHLCV table: one row per bar."""
    blocks: list[str] = []
    for symbol, candles in market_data.items():
        if not candles:
            continue
        header = f"{symbol} (O  H  L  C  Vol)"
        rows = []
        for c in candles[-20:]:
            label = _bar_label(c) if hourly else str(c.get("date", c.get("timestamp", "")))[:10]
            o = float(c.get("open", 0))
            h = float(c.get("high", 0))
            lv = float(c.get("low", 0))
            cl = float(c.get("close", 0))
            vol = int(c.get("volume", 0))
            rows.append(f"  {label}  {o:.2f}  {h:.2f}  {lv:.2f}  {cl:.2f}  {vol:,}")
        blocks.append(header + "\n" + "\n".join(rows))
    return "\n\n".join(blocks)


def _turn_header(current_date: str, tick: dict[str, Any] | None) -> str:
    tick = tick or {}
    bar_ts = tick.get("bar_ts")
    session_bar = tick.get("session_bar")
    session_bars = tick.get("session_bars")
    if bar_ts and session_bar and session_bars:
        label = str(bar_ts).replace("T", " ").replace("+00:00", " UTC")
        return f"=== TRADING TURN: {current_date} {label} (bar {session_bar}/{session_bars}) ==="
    if bar_ts:
        return f"=== TRADING TURN: {current_date} {bar_ts} ==="
    return f"=== TRADING TURN: {current_date} ==="


def build_trading_context(
    current_date: str,
    portfolio: dict[str, Any],
    market_data: dict[str, list[dict]],
    tick: dict[str, Any] | None = None,
    hourly_data: dict[str, list[dict]] | None = None,
) -> str:
    """Format portfolio and OHLCV snapshots for LLM context."""
    cash = float(portfolio.get("cash", 0))
    total_value = float(portfolio.get("total_value", cash))
    pnl_pct = float(portfolio.get("total_return_pct", portfolio.get("pnl_pct", 0)))
    holdings: list[dict] = portfolio.get("holdings", [])

    sections = [
        _turn_header(current_date, tick),
        "\nPORTFOLIO\n"
        f"  Cash available : ₹{cash:,.2f}\n"
        f"  Total value    : ₹{total_value:,.2f}\n"
        f"  Overall P&L    : {pnl_pct:+.2f}%\n"
        "\nHOLDINGS\n"
        f"{_fmt_holdings(holdings)}\n"
        "\nMARKET DATA (last 20 trading days)\n"
        f"{_fmt_ohlcv(market_data)}\n",
    ]
    if hourly_data:
        sections.append(
            "TODAY / RECENT HOURLY BARS (completed only)\n"
            f"{_fmt_ohlcv(hourly_data, hourly=True)}\n"
        )
    sections.append(
        "Output ONLY a JSON array of orders, e.g.:\n"
        '[{"symbol":"RELIANCE.NS","side":"buy","order_type":"market","quantity":10}]\n'
        "Return [] to hold.\n"
    )
    return "".join(sections)
