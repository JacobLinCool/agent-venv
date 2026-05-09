from __future__ import annotations

import os
import stat
from pathlib import Path

import pytest

from agent_venv import Environment, EnvironmentSpec, errors


def test_ephemeral_create_destroy(tmp_path: Path):
    env = Environment.ephemeral(EnvironmentSpec(env_overrides={"FOO": "$EPHEMERAL_HOME"}))
    try:
        assert env.path.exists()
        assert env.env_overrides == {"FOO": str(env.path)}
        assert env.kind == "ephemeral"
    finally:
        env.destroy()
    assert not env.path.exists()


def test_ephemeral_seed_files():
    spec = EnvironmentSpec(seed_files={"a.txt": "hi", "nested/b.txt": "yo"})
    with Environment.ephemeral(spec) as env:
        assert (env.path / "a.txt").read_text() == "hi"
        assert (env.path / "nested" / "b.txt").read_text() == "yo"


def test_ephemeral_no_home_in_env_overrides():
    spec = EnvironmentSpec(env_overrides={"CLAUDE_CONFIG_DIR": "$EPHEMERAL_HOME"})
    with Environment.ephemeral(spec) as env:
        assert "CLAUDE_CONFIG_DIR" in env.env_overrides
        assert "HOME" not in env.env_overrides


def test_ephemeral_credentials_default_mode_0600():
    spec = EnvironmentSpec(credentials={".credentials.json": '{"k":"v"}'})
    with Environment.ephemeral(spec) as env:
        path = env.path / ".credentials.json"
        assert path.exists()
        if os.name == "posix":
            mode = stat.S_IMODE(path.stat().st_mode)
            assert mode == 0o600


def test_ephemeral_explicit_file_mode_overrides_default():
    spec = EnvironmentSpec(
        credentials={".credentials.json": "x"},
        file_modes={".credentials.json": 0o644},
    )
    with Environment.ephemeral(spec) as env:
        if os.name == "posix":
            mode = stat.S_IMODE((env.path / ".credentials.json").stat().st_mode)
            assert mode == 0o644


def test_seed_path_escape_rejected():
    spec = EnvironmentSpec(seed_files={"../escape.txt": "bad"})
    with pytest.raises(errors.ProfileSetupFailedError):
        Environment.ephemeral(spec)


def test_persistent_create_or_attach_idempotent(tmp_path: Path):
    name = "test-env-1"
    spec = EnvironmentSpec(seed_files={"hello.txt": "world"})
    env1 = Environment.create_or_attach(name, spec, registry_root=tmp_path)
    env2 = Environment.create_or_attach(name, spec, registry_root=tmp_path)
    assert env1.path == env2.path
    assert (env2.path / "hello.txt").read_text() == "world"
    assert env1.kind == "persistent"


def test_persistent_attach_returns_same_path(tmp_path: Path):
    spec = EnvironmentSpec(env_overrides={"FOO": "$EPHEMERAL_HOME"})
    env1 = Environment.create_or_attach("env-X", spec, registry_root=tmp_path)
    env2 = Environment.attach("env-X", registry_root=tmp_path)
    assert env1.path == env2.path
    assert env2.env_overrides == env1.env_overrides


def test_attach_missing_raises(tmp_path: Path):
    with pytest.raises(errors.EnvironmentNotFoundError):
        Environment.attach("nope", registry_root=tmp_path)


def test_persistent_list_and_destroy_by_name(tmp_path: Path):
    spec = EnvironmentSpec()
    Environment.create_or_attach("a", spec, registry_root=tmp_path)
    Environment.create_or_attach("b", spec, registry_root=tmp_path)
    assert sorted(Environment.list(registry_root=tmp_path)) == ["a", "b"]
    Environment.destroy_by_name("a", registry_root=tmp_path)
    assert Environment.list(registry_root=tmp_path) == ["b"]


def test_destroy_by_name_missing(tmp_path: Path):
    with pytest.raises(errors.EnvironmentNotFoundError):
        Environment.destroy_by_name("ghost", registry_root=tmp_path)


def test_attach_mismatch(tmp_path: Path):
    spec_a = EnvironmentSpec(adapter_id="claude-code")
    spec_b = EnvironmentSpec(adapter_id="codex")
    Environment.create_or_attach("multi", spec_a, registry_root=tmp_path)
    with pytest.raises(errors.AdapterMismatchError):
        Environment.create_or_attach("multi", spec_b, registry_root=tmp_path)


def test_persistent_destroy_handle_removes_index_entry(tmp_path: Path):
    env = Environment.create_or_attach("zz", EnvironmentSpec(), registry_root=tmp_path)
    env.destroy()
    assert "zz" not in Environment.list(registry_root=tmp_path)
    assert not env.path.exists()


def test_event_log_full_sequence_ephemeral():
    seen: list[str] = []
    spec = EnvironmentSpec(seed_files={"a.txt": "1"}, credentials={"c": "x"})
    with Environment.ephemeral(spec, on_event=lambda e: seen.append(e.kind)) as env:
        pass
    assert "env.created" in seen
    assert "profile.materialized" in seen
    assert "credentials.copied" in seen
    assert "env.destroyed" in seen
