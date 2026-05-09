from __future__ import annotations

from pathlib import Path

import pytest

from agent_venv import AsyncEnvironment, EnvironmentSpec, errors


@pytest.mark.asyncio
async def test_async_ephemeral():
    spec = EnvironmentSpec(env_overrides={"FOO": "$EPHEMERAL_HOME"})
    async with await AsyncEnvironment.ephemeral(spec) as env:
        assert env.path.exists()
        assert env.env_overrides["FOO"] == str(env.path)


@pytest.mark.asyncio
async def test_async_persistent_idempotent(tmp_path: Path):
    spec = EnvironmentSpec(seed_files={"x.txt": "1"})
    e1 = await AsyncEnvironment.create_or_attach("E", spec, registry_root=tmp_path)
    e2 = await AsyncEnvironment.create_or_attach("E", spec, registry_root=tmp_path)
    assert e1.path == e2.path


@pytest.mark.asyncio
async def test_async_attach_missing(tmp_path: Path):
    with pytest.raises(errors.EnvironmentNotFoundError):
        await AsyncEnvironment.attach("nope", registry_root=tmp_path)
