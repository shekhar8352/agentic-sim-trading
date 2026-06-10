import pytest

from prompts.personalities import (
    build_system_prompt,
    get_personality,
    list_personalities,
)


def test_list_personalities_includes_risk_taker():
    ids = {p["id"] for p in list_personalities()}
    assert "risk_taker" in ids
    assert "momentum" in ids
    assert "mean_reversion" in ids


def test_build_system_prompt_balanced_is_base_only():
    base = build_system_prompt("balanced")
    assert "Trading personality" not in base


def test_build_system_prompt_risk_taker_adds_modifier():
    prompt = build_system_prompt("risk_taker")
    assert "risk-taking" in prompt.lower()
    assert "30%" in prompt


def test_build_system_prompt_custom_override_wins():
    prompt = build_system_prompt("risk_taker", custom_override="Only trade TCS.")
    assert prompt == "Only trade TCS."


def test_unknown_personality_raises():
    with pytest.raises(ValueError, match="unknown personality"):
        get_personality("not-a-personality")
