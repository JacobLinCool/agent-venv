"""Profile materialization: write spec into a directory."""

from __future__ import annotations

import os
import shutil
from pathlib import Path

from . import errors as _errors
from .events import EventLog
from .spec import EnvironmentSpec


def _write_files(
    base: Path,
    files: dict[str, str],
    file_modes: dict[str, int],
    default_mode: int | None = None,
) -> tuple[int, int]:
    base = base.resolve()
    count = 0
    total = 0
    for rel, content in files.items():
        rel_path = Path(rel)
        if rel_path.is_absolute():
            raise _errors.ProfileSetupFailedError(
                f"path must be relative: {rel!r}", reason="absolute_path"
            )
        for comp in rel_path.parts:
            if comp in ("..",):
                raise _errors.ProfileSetupFailedError(
                    f"path escapes profile: {rel!r}", reason="parent_traversal"
                )
        target = (base / rel_path).resolve()
        try:
            target.relative_to(base)
        except ValueError as exc:
            raise _errors.ProfileSetupFailedError(
                f"path escapes profile: {rel!r}", reason="resolved_escape"
            ) from exc
        target.parent.mkdir(parents=True, exist_ok=True)
        encoded = content.encode("utf-8")
        target.write_bytes(encoded)
        mode = file_modes.get(rel, default_mode)
        if mode is not None:
            os.chmod(target, mode)
        count += 1
        total += len(encoded)
    return count, total


def materialize(
    profile_dir: Path,
    spec: EnvironmentSpec,
    log: EventLog,
    *,
    skip_seed_if_exists: bool = False,
) -> dict[str, str]:
    """Write seed_files + credentials into ``profile_dir`` per ``spec``.

    Returns the resolved env_overrides ($EPHEMERAL_HOME placeholder replaced
    with the absolute path of profile_dir).

    If ``skip_seed_if_exists`` is True (used when reattaching to a
    pre-existing persistent env), seed_files and credentials are NOT
    rewritten — only env_overrides are recomputed.
    """

    profile_dir = profile_dir.resolve()
    profile_dir.mkdir(parents=True, exist_ok=True)

    if not skip_seed_if_exists and spec.seed_files:
        try:
            count, total = _write_files(profile_dir, spec.seed_files, spec.file_modes)
        except _errors.ProfileSetupFailedError:
            raise
        except OSError as exc:
            raise _errors.ProfileSetupFailedError(
                f"writing seed files: {exc}", reason="write_failed"
            ) from exc
        log.emit("profile.materialized", file_count=count, total_bytes=total)

    if not skip_seed_if_exists and spec.credentials:
        try:
            count, _ = _write_files(
                profile_dir, spec.credentials, spec.file_modes, default_mode=0o600
            )
        except _errors.ProfileSetupFailedError:
            raise
        except OSError as exc:
            raise _errors.ProfileSetupFailedError(
                f"writing credentials: {exc}", reason="write_failed"
            ) from exc
        log.emit("credentials.copied", file_count=count)

    home_str = str(profile_dir)
    return {k: v.replace("$EPHEMERAL_HOME", home_str) for k, v in spec.env_overrides.items()}


def remove_dir(path: Path) -> bool:
    if not path.exists():
        return True
    try:
        shutil.rmtree(path)
        return True
    except OSError:
        return False
