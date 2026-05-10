"""
Load Nifty 50 EOD OHLCV from Yahoo Finance into Postgres.
Rules: docs/rules.md (frozen universe, adjusted prices, Jan 2022 – Dec 2024).
Roadmap: docs/rroadmap.md Step 5.
"""

from __future__ import annotations

import argparse
import os
import sys

import psycopg2
import pandas as pd
import yfinance as yf

# Frozen Nifty 50 as of simulation rules (Jan 2022). Source: docs/rules.md
NIFTY_50 = (
    "ADANIENT.NS",
    "ADANIPORTS.NS",
    "APOLLOHOSP.NS",
    "ASIANPAINT.NS",
    "AXISBANK.NS",
    "BAJAJ-AUTO.NS",
    "BAJFINANCE.NS",
    "BAJAJFINSV.NS",
    "BPCL.NS",
    "BHARTIARTL.NS",
    "BRITANNIA.NS",
    "CIPLA.NS",
    "COALINDIA.NS",
    "DIVISLAB.NS",
    "DRREDDY.NS",
    "EICHERMOT.NS",
    "GRASIM.NS",
    "HCLTECH.NS",
    "HDFCBANK.NS",
    "HDFCLIFE.NS",
    "HEROMOTOCO.NS",
    "HINDALCO.NS",
    "HINDUNILVR.NS",
    "ICICIBANK.NS",
    "ITC.NS",
    "INDUSINDBK.NS",
    "INFY.NS",
    "JSWSTEEL.NS",
    "KOTAKBANK.NS",
    "LT.NS",
    "LTIM.NS",
    "M&M.NS",
    "MARUTI.NS",
    "NESTLEIND.NS",
    "NTPC.NS",
    "ONGC.NS",
    "POWERGRID.NS",
    "RELIANCE.NS",
    "SBILIFE.NS",
    "SBIN.NS",
    "SUNPHARMA.NS",
    "TATACONSUM.NS",
    "TATAMOTORS.NS",
    "TATASTEEL.NS",
    "TCS.NS",
    "TECHM.NS",
    "TITAN.NS",
    "ULTRACEMCO.NS",
    "UPL.NS",
    "WIPRO.NS",
)

DEFAULT_DATABASE_URL = "postgresql://admin:secret@localhost:5432/tradingsim"
DATA_START = "2022-01-01"
DATA_END = "2025-01-01"  # exclusive upper bound → includes bars through Dec 2024


def flatten_columns(df: pd.DataFrame) -> pd.DataFrame:
    if isinstance(df.columns, pd.MultiIndex):
        out = df.copy()
        lvl0 = df.columns.get_level_values(0)
        out.columns = lvl0
        return out
    return df


def fetch_history_rows(ticker: str) -> list[tuple]:
    """Return [(date, open, high, low, close, volume), ...]."""
    raw = yf.Ticker(ticker).history(
        start=DATA_START,
        end=DATA_END,
        interval="1d",
        auto_adjust=True,
    )
    if raw.empty:
        return []

    df = flatten_columns(raw)
    colmap = {str(c).lower(): c for c in df.columns}
    for key in ("open", "high", "low", "close", "volume"):
        if key not in colmap:
            print(f"  ! missing column '{key}' for {ticker}: {list(df.columns)}", file=sys.stderr)
            return []

    o_c, h_c, l_c, c_c, v_c = (
        colmap["open"],
        colmap["high"],
        colmap["low"],
        colmap["close"],
        colmap["volume"],
    )

    rows: list[tuple] = []
    for ts, row in df.iterrows():
        o = float(row[o_c])
        h = float(row[h_c])
        l_ = float(row[l_c])
        c = float(row[c_c])
        vol = row[v_c]
        if pd.isna(o) or pd.isna(h) or pd.isna(l_) or pd.isna(c):
            continue
        v = int(vol) if pd.notna(vol) else 0
        d = ts.date() if hasattr(ts, "date") else pd.Timestamp(ts).date()
        rows.append((d, o, h, l_, c, v))
    return rows


def ensure_stocks(cur, symbols: tuple[str, ...]) -> None:
    for sym in symbols:
        base = sym.replace(".NS", "")
        cur.execute(
            """
            INSERT INTO stocks (symbol, name, sector, is_active)
            VALUES (%s, %s, NULL, TRUE)
            ON CONFLICT (symbol) DO NOTHING
            """,
            (sym, base),
        )


def insert_ohlcv(cur, symbol: str, rows: list[tuple]) -> int:
    count = 0
    for d, o, h, l_, c, v in rows:
        cur.execute(
            """
            INSERT INTO ohlcv (symbol, date, open, high, low, close, volume)
            VALUES (%s, %s, %s, %s, %s, %s, %s)
            ON CONFLICT (symbol, date) DO NOTHING
            """,
            (symbol, d, o, h, l_, c, v),
        )
        count += cur.rowcount
    return count


def ingest(
    database_url: str,
    symbols: tuple[str, ...],
    *,
    dry_run: bool,
) -> None:
    if dry_run:
        t = symbols[0]
        rows = fetch_history_rows(t)
        print(f"dry-run: {t} -> {len(rows)} rows (showing up to 3)")
        for r in rows[:3]:
            print(" ", r)
        return

    conn = psycopg2.connect(database_url)
    try:
        cur = conn.cursor()
        ensure_stocks(cur, symbols)
        conn.commit()

        total_ohlcv = 0
        for ticker in symbols:
            print(f"Fetching {ticker}...")
            rows = fetch_history_rows(ticker)
            if not rows:
                print(f"  ! no rows for {ticker}", file=sys.stderr)
                continue
            inserted = insert_ohlcv(cur, ticker, rows)
            conn.commit()
            total_ohlcv += inserted
            print(f"  inserted {inserted} new ohlcv rows ({len(rows)} bars from Yahoo)")

        print(f"Done. New ohlcv rows this run: {total_ohlcv}")
        cur.close()
    finally:
        conn.close()


def main() -> None:
    p = argparse.ArgumentParser(description="Ingest Nifty 50 OHLCV via yfinance into Postgres.")
    p.add_argument(
        "--database-url",
        default=os.environ.get("DATABASE_URL", DEFAULT_DATABASE_URL),
        help="Postgres URL (default: DATABASE_URL or localhost tradingsim)",
    )
    p.add_argument(
        "--dry-run",
        action="store_true",
        help="Fetch first symbol only, print sample rows, no database writes",
    )
    args = p.parse_args()

    if len(NIFTY_50) != 50:
        print("NIFTY_50 must contain exactly 50 symbols.", file=sys.stderr)
        sys.exit(1)

    ingest(args.database_url, NIFTY_50, dry_run=args.dry_run)


if __name__ == "__main__":
    main()
