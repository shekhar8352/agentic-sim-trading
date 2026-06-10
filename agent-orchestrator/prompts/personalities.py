"""Trading personalities — prompt modifiers for LLM agents (roadmap Step 19)."""

from __future__ import annotations

from dataclasses import dataclass

from prompts.system import TRADING_SYSTEM_PROMPT


@dataclass(frozen=True)
class Personality:
    id: str
    label: str
    description: str
    prompt_addon: str
    applies_to: str  # "llm" | "all"


PERSONALITIES: tuple[Personality, ...] = (
    Personality(
        id="balanced",
        label="Balanced",
        description="Moderate risk, diversified sizing, default competitor baseline.",
        prompt_addon="",
        applies_to="llm",
    ),
    Personality(
        id="risk_taker",
        label="Risk Taker",
        description="Aggressive sizing, chases breakouts, tolerates volatility for higher upside.",
        prompt_addon="""\
You are a **risk-taking** trader. Prioritize upside over capital preservation.
- Size up to **30%** of portfolio value per position when conviction is high.
- Buy strong momentum breakouts even if volatility is elevated.
- Cut losers quickly (2-day decline) but add to winners on continued strength.
- Prefer action over waiting — deploy idle cash when any reasonable signal exists.
- Accept higher turnover; do not over-diversify into tiny positions.""",
        applies_to="llm",
    ),
    Personality(
        id="conservative",
        label="Conservative",
        description="Capital preservation first, small positions, waits for confirmation.",
        prompt_addon="""\
You are a **conservative** trader. Protect capital above all else.
- Cap any single position at **10%** of portfolio value.
- Require clear multi-day confirmation before buying (uptrend + above 10-day average).
- Sell on the first sign of weakness (2-day decline or breakdown below recent lows).
- Keep **≥30% cash** unless multiple high-conviction setups align.
- Prefer quality large-cap names; skip marginal or noisy signals.""",
        applies_to="llm",
    ),
    Personality(
        id="momentum",
        label="Momentum",
        description="Buys stocks trending up over the last 10 days; rides winners.",
        prompt_addon="""\
You are a **momentum** trader (Step 19 strategy).
- Rank symbols by 10-day price change; prefer the strongest uptrends.
- BUY names where close > 10-day average AND 10-day return > +3%.
- SELL holdings that lose momentum (close below 10-day average for 2+ days).
- Concentrate in top 2–3 momentum leaders; size up to **25%** per leader.
- Do not buy falling knives or range-bound stocks.""",
        applies_to="llm",
    ),
    Personality(
        id="mean_reversion",
        label="Mean Reversion",
        description="Buys oversold dips; sells when price recovers toward the mean.",
        prompt_addon="""\
You are a **mean-reversion** trader (Step 19 strategy).
- Look for oversold conditions: close **≥5% below** the 10-day average with stable volume.
- BUY dips in liquid Nifty names; expect bounce toward the mean.
- SELL when price recovers to or above the 10-day average (+2% buffer).
- Avoid catching prolonged downtrends — skip if 5-day trend is still sharply negative.
- Size modestly (**≤15%** per position); scale out on recovery.""",
        applies_to="llm",
    ),
    Personality(
        id="contrarian",
        label="Contrarian",
        description="Fades short-term extremes — buys fear, trims euphoria.",
        prompt_addon="""\
You are a **contrarian** trader.
- BUY when recent panic: sharp 3–5 day selloff (>6%) in otherwise liquid names.
- SELL or trim when short-term euphoria: 5-day rally >8% without pullback.
- Fade crowded momentum; prefer unloved names with improving last-day candle.
- Stay patient — [] is fine when no extreme is present.
- Position size **≤20%**; contrarian bets need room to be wrong briefly.""",
        applies_to="llm",
    ),
)

_PERSONALITY_BY_ID = {p.id: p for p in PERSONALITIES}
DEFAULT_PERSONALITY_ID = "balanced"


def get_personality(personality_id: str) -> Personality:
    pid = (personality_id or DEFAULT_PERSONALITY_ID).strip().lower()
    if pid not in _PERSONALITY_BY_ID:
        raise ValueError(f"unknown personality: {personality_id}")
    return _PERSONALITY_BY_ID[pid]


def list_personalities(*, for_provider: str | None = None) -> list[dict]:
    """Return personality catalog for API / dashboard."""
    rows: list[dict] = []
    for p in PERSONALITIES:
        if for_provider == "custom" and p.applies_to != "all":
            continue
        rows.append(
            {
                "id": p.id,
                "label": p.label,
                "description": p.description,
                "applies_to": p.applies_to,
            }
        )
    return rows


def build_system_prompt(
    personality_id: str | None = None,
    *,
    custom_override: str | None = None,
) -> str:
    """Compose base trading prompt with optional personality modifier."""
    if custom_override and custom_override.strip():
        return custom_override.strip()

    base = TRADING_SYSTEM_PROMPT
    pid = (personality_id or DEFAULT_PERSONALITY_ID).strip().lower()
    if pid == DEFAULT_PERSONALITY_ID:
        return base

    personality = get_personality(pid)
    if not personality.prompt_addon:
        return base

    return f"{base.rstrip()}\n\n## Trading personality: {personality.label}\n\n{personality.prompt_addon.strip()}\n"
