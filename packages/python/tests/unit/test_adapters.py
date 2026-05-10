from __future__ import annotations

from pathlib import Path

from agent_venv import ClaudeCode, Codex, Environment


def test_claude_code_spec_no_creds():
    spec = ClaudeCode(load_credentials=False).build_spec()
    assert spec.adapter_id == "claude-code"
    assert spec.env_overrides == {"CLAUDE_CONFIG_DIR": "$EPHEMERAL_HOME"}
    assert ".claude.json" in spec.seed_files
    assert ".credentials.json" not in spec.credentials


def test_claude_code_argv():
    adapter = ClaudeCode(
        model="claude-haiku-4-5-20251001", reasoning_effort="high", load_credentials=False
    )
    argv = adapter.build_argv("hi")
    assert argv[0] == "claude"
    assert "--print" in argv
    assert "claude-haiku-4-5-20251001" in argv
    assert "--effort" in argv
    assert "high" in argv


def test_codex_spec_no_creds():
    spec = Codex(load_credentials=False).build_spec()
    assert spec.adapter_id == "codex"
    assert spec.env_overrides == {"CODEX_HOME": "$EPHEMERAL_HOME"}


def test_codex_argv_with_workspace():
    adapter = Codex(model="gpt-5", load_credentials=False)
    argv = adapter.build_argv("hi", workspace=Path("/tmp/ws"))
    assert argv[0] == "codex"
    assert "exec" in argv
    assert "gpt-5" in argv
    assert "--cd" in argv
    assert "/tmp/ws" in argv


def test_environment_via_adapter_no_creds():
    adapter = ClaudeCode(load_credentials=False)
    with Environment.ephemeral(adapter=adapter) as env:
        assert env.adapter_id == "claude-code"
        assert env.env_overrides["CLAUDE_CONFIG_DIR"] == str(env.path)
        assert "HOME" not in env.env_overrides
        assert (env.path / ".claude.json").read_text().startswith("{")
