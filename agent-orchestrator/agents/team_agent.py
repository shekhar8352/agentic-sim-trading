"""Multi-agent trading desk: analyst / risk / strategist brief the head trader."""

from __future__ import annotations

import logging
from typing import Any, Sequence

from agents.base_agent import BaseAgent
from agents.decisions import is_valid_llm_output, parse_orders_json
from agents.llm_agent import LLMAgent, _is_retryable_llm_error
from market_client.models import Order
from prompts.roles import (
    DEFAULT_TEAM_ROLES,
    normalize_team_roles,
    system_prompt_for_role,
)

logger = logging.getLogger(__name__)

_LLM_MAX_ATTEMPTS = 2


class TradingTeamAgent(BaseAgent):
    """One portfolio, many roles: advisory teammates then a head that places orders.

    Shares the wrapped LLM agent's market client / credentials. On each turn:

    1. **Analyst** — market trend briefing
    2. **Risk Officer** + **Strategist** (in parallel) — constraints and trade ideas
    3. **Head Trader** — final JSON orders
    """

    def __init__(
        self,
        backend: LLMAgent,
        *,
        roles: Sequence[str] | None = None,
    ):
        if not isinstance(backend, LLMAgent):
            raise TypeError("TradingTeamAgent requires an LLMAgent backend")

        # Reuse backend identity + client (no second MarketClient).
        self._backend = backend
        self.agent_id = backend.agent_id
        self.sim_id = backend.sim_id
        self.config = backend.config
        self.client = backend.client
        self._last_portfolio: dict[str, Any] = {}
        self._last_market_data: dict[str, list[dict]] = {}
        self.roles = normalize_team_roles(roles)
        self._last_briefing: dict[str, str] = {}
        self._head_personality_prompt = backend.system_prompt

    @property
    def system_prompt(self) -> str:
        """Head personality / house rules (used when updating prompts live)."""
        return self._head_personality_prompt

    @system_prompt.setter
    def system_prompt(self, value: str) -> None:
        self._head_personality_prompt = value
        self._backend.system_prompt = value

    @property
    def backend(self) -> LLMAgent:
        return self._backend

    async def build_context(self, current_date: str) -> str:
        text = await self._backend.build_context(current_date)
        self._last_portfolio = self._backend._last_portfolio
        self._last_market_data = self._backend._last_market_data
        return text

    async def _complete_as(self, role: str, user_content: str) -> str:
        prompt = system_prompt_for_role(
            role,
            head_personality_prompt=self._head_personality_prompt,
        )
        original = self._backend.system_prompt
        self._backend.system_prompt = prompt
        try:
            for attempt in range(1, _LLM_MAX_ATTEMPTS + 1):
                try:
                    return await self._backend.complete(user_content)
                except Exception as exc:
                    if attempt < _LLM_MAX_ATTEMPTS and _is_retryable_llm_error(exc):
                        logger.warning(
                            "agent=%s role=%s llm retry attempt=%d: %s",
                            self.agent_id,
                            role,
                            attempt,
                            exc,
                        )
                        continue
                    logger.warning(
                        "agent=%s role=%s llm error: %s",
                        self.agent_id,
                        role,
                        exc,
                    )
                    return ""
        finally:
            self._backend.system_prompt = original
        return ""

    def _advisory_user_payload(
        self,
        context: str,
        *,
        analyst_report: str | None = None,
        risk_report: str | None = None,
    ) -> str:
        parts = [context.rstrip(), "", "=== DESK BRIEFING SO FAR ==="]
        if analyst_report:
            parts.append("Analyst report:")
            parts.append(analyst_report.strip() or "(empty)")
        if risk_report:
            parts.append("Risk officer memo:")
            parts.append(risk_report.strip() or "(empty)")
        if not analyst_report and not risk_report:
            parts.append("(no prior teammate reports yet)")
        return "\n".join(parts)

    def _head_user_payload(self, context: str, briefing: dict[str, str]) -> str:
        parts = [
            context.rstrip(),
            "",
            "=== TEAMMATE REPORTS (synthesize these) ===",
        ]
        for role in self.roles:
            if role == "head":
                continue
            label = role.replace("_", " ").title()
            parts.append(f"## {label}")
            parts.append((briefing.get(role) or "(no report)").strip())
            parts.append("")
        parts.append(
            "Emit ONLY the final JSON array of orders (or []). "
            "Do not repeat teammate JSON objects."
        )
        return "\n".join(parts)

    async def _deliberate(self, context: str) -> dict[str, str]:
        briefing: dict[str, str] = {}
        advisory = [r for r in self.roles if r != "head"]

        if "analyst" in advisory:
            briefing["analyst"] = await self._complete_as(
                "analyst",
                self._advisory_user_payload(context),
            )
            logger.info(
                "agent=%s role=analyst report_len=%d",
                self.agent_id,
                len(briefing["analyst"]),
            )

        parallel_roles = [r for r in advisory if r != "analyst"]
        if parallel_roles:
            analyst_report = briefing.get("analyst")

            async def _run(role: str) -> tuple[str, str]:
                payload = self._advisory_user_payload(
                    context,
                    analyst_report=analyst_report,
                )
                text = await self._complete_as(role, payload)
                return role, text

            # Roles after analyst can run concurrently — each uses its own
            # temporary system_prompt swap, so serialize if sharing one backend.
            # Sequential is safer with a shared backend.complete(); keep order.
            for role in parallel_roles:
                role_name, text = await _run(role)
                briefing[role_name] = text
                logger.info(
                    "agent=%s role=%s report_len=%d",
                    self.agent_id,
                    role_name,
                    len(text),
                )

        if "head" in self.roles:
            head_raw = await self._complete_as(
                "head",
                self._head_user_payload(context, briefing),
            )
            briefing["head"] = head_raw
            logger.info(
                "agent=%s role=head output_len=%d",
                self.agent_id,
                len(head_raw),
            )

        return briefing

    async def decide(self, context: str) -> list[Order]:
        try:
            briefing = await self._deliberate(context)
        except Exception as exc:
            logger.warning("agent=%s team deliberate failed: %s", self.agent_id, exc)
            self._last_briefing = {}
            return []

        self._last_briefing = briefing
        raw = briefing.get("head", "")
        orders = parse_orders_json(raw)
        if not orders and raw.strip() and not is_valid_llm_output(raw, orders):
            logger.warning(
                "agent=%s head output unparseable, holding (raw_len=%d)",
                self.agent_id,
                len(raw),
            )
        return orders

    async def act(self, current_date: str) -> dict[str, Any]:
        placed = await self.run_turn(current_date)
        return {
            "orders": placed,
            "briefing": dict(self._last_briefing),
            "roles": list(self.roles),
        }


def wrap_with_team(
    agent: BaseAgent,
    *,
    team_mode: bool,
    roles: Sequence[str] | None = None,
) -> BaseAgent:
    """Optionally wrap an LLM agent in a multi-role trading desk."""
    if not team_mode:
        return agent
    if not isinstance(agent, LLMAgent):
        logger.warning(
            "team_mode ignored for non-LLM agent provider=%s",
            agent.config.get("provider"),
        )
        return agent
    if isinstance(agent, TradingTeamAgent):
        return agent
    resolved = normalize_team_roles(roles) if roles is not None else DEFAULT_TEAM_ROLES
    return TradingTeamAgent(agent, roles=resolved)
