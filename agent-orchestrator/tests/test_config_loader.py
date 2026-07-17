from pathlib import Path

from app.config_loader import load_agents_config


def test_load_agents_config_missing_file():
    cfg = load_agents_config(Path("/nonexistent/agents.yaml"))
    assert cfg.agents == []


def test_load_agents_config_sample(tmp_path: Path):
    p = tmp_path / "agents.yaml"
    p.write_text(
        "go_service_url: http://localhost:8070\n"
        "agents:\n"
        "  - name: a\n"
        "    provider: custom\n"
        "    model: x\n"
    )
    cfg = load_agents_config(p)
    assert cfg.go_service_url == "http://localhost:8070"
    assert len(cfg.agents) == 1
    assert cfg.agents[0].name == "a"


def test_load_agents_config_team_block(tmp_path: Path):
    p = tmp_path / "agents.yaml"
    p.write_text(
        "go_service_url: http://localhost:8070\n"
        "agents:\n"
        "  - name: desk\n"
        "    provider: gpt\n"
        "    model: gpt-4o\n"
        "    team_mode: true\n"
        "    team:\n"
        "      enabled: true\n"
        "      roles: [analyst, head]\n"
    )
    cfg = load_agents_config(p)
    assert cfg.agents[0].team_mode is True
    assert cfg.agents[0].team is not None
    assert cfg.agents[0].team.roles == ["analyst", "head"]
