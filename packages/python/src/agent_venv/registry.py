"""Persistent environment registry.

See spec/registry.md.
"""

from __future__ import annotations

import dataclasses
import datetime as _dt
import hashlib
import json
import os
import shutil
import time
from pathlib import Path
from typing import Any

from . import errors as _errors


REGISTRY_SCHEMA_VERSION = 1


def default_registry_root() -> Path:
    """Resolve the default registry path per the spec."""

    env_override = os.environ.get("AGENT_VENV_REGISTRY_ROOT")
    if env_override:
        return Path(env_override)
    xdg = os.environ.get("XDG_DATA_HOME")
    if xdg:
        return Path(xdg) / "agent-venv" / "envs"
    return Path.home() / ".local" / "share" / "agent-venv" / "envs"


def slug_for(name: str) -> str:
    return hashlib.sha256(name.encode("utf-8")).hexdigest()[:16]


@dataclasses.dataclass
class IndexEntry:
    name: str
    relative_dir: str  # relative to registry_root


@dataclasses.dataclass
class Metadata:
    schema_version: int
    name: str
    adapter_id: str
    created_at: str
    env_overrides: dict[str, str]
    credentials_loaded: bool
    credentials_loaded_at: str | None

    def to_dict(self) -> dict[str, Any]:
        return dataclasses.asdict(self)

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> Metadata:
        return cls(
            schema_version=int(data.get("schema_version", 1)),
            name=str(data["name"]),
            adapter_id=str(data["adapter_id"]),
            created_at=str(data.get("created_at", "")),
            env_overrides=dict(data.get("env_overrides") or {}),
            credentials_loaded=bool(data.get("credentials_loaded", False)),
            credentials_loaded_at=data.get("credentials_loaded_at"),
        )


class Registry:
    """File-system-backed registry of persistent environments.

    Locking is advisory via a simple lock file + ``fcntl.flock`` on Unix.
    """

    def __init__(self, root: Path):
        self.root = root.resolve() if root.exists() else root.absolute()
        self._index_path = self.root / "index.json"
        self._envs_dir = self.root / "envs"
        self._lock_path = self.root / ".lock"

    # ------------------------------------------------------------------
    # Locking
    # ------------------------------------------------------------------

    def _acquire_lock(self) -> Any:
        self.root.mkdir(parents=True, exist_ok=True)
        # Lazy import: only Unix has fcntl. On Windows we don't lock in v0.
        try:
            import fcntl

            f = open(self._lock_path, "w")
            for _ in range(50):
                try:
                    fcntl.flock(f.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
                    return f
                except OSError:
                    time.sleep(0.1)
            raise _errors.RegistryUnavailableError(
                "could not acquire registry lock", path=str(self._lock_path)
            )
        except ImportError:
            return open(self._lock_path, "w")

    def _release_lock(self, fp: Any) -> None:
        try:
            import fcntl

            fcntl.flock(fp.fileno(), fcntl.LOCK_UN)
        except (ImportError, OSError):
            pass
        try:
            fp.close()
        except OSError:
            pass

    # ------------------------------------------------------------------
    # Index
    # ------------------------------------------------------------------

    def _read_index(self) -> dict[str, str]:
        if not self._index_path.exists():
            return {}
        try:
            data = json.loads(self._index_path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError) as exc:
            raise _errors.RegistryUnavailableError(
                f"reading index: {exc}", path=str(self._index_path)
            ) from exc
        entries = data.get("entries") or {}
        return {str(k): str(v) for k, v in entries.items()}

    def _write_index(self, entries: dict[str, str]) -> None:
        payload = {
            "schema_version": REGISTRY_SCHEMA_VERSION,
            "entries": dict(entries),
        }
        self.root.mkdir(parents=True, exist_ok=True)
        tmp = self._index_path.with_suffix(".json.tmp")
        tmp.write_text(json.dumps(payload, indent=2), encoding="utf-8")
        os.replace(tmp, self._index_path)

    # ------------------------------------------------------------------
    # Public ops
    # ------------------------------------------------------------------

    def list_names(self) -> list[str]:
        return sorted(self._read_index().keys())

    def lookup(self, name: str) -> tuple[Path, Metadata] | None:
        entries = self._read_index()
        rel = entries.get(name)
        if rel is None:
            return None
        env_dir = (self.root / rel).resolve() if (self.root / rel).exists() else self.root / rel
        meta_path = env_dir / "metadata.json"
        if not meta_path.exists():
            return None
        try:
            meta = Metadata.from_dict(json.loads(meta_path.read_text(encoding="utf-8")))
        except (OSError, json.JSONDecodeError, KeyError) as exc:
            raise _errors.RegistryUnavailableError(
                f"reading metadata for {name!r}: {exc}", path=str(meta_path)
            ) from exc
        return env_dir, meta

    def reserve_or_get(
        self, name: str, adapter_id: str
    ) -> tuple[Path, Metadata, bool]:
        """Returns ``(env_dir, metadata, created)``.

        If the name is new, allocates a fresh env_dir, writes a stub metadata,
        and registers it in the index. If the name already exists, validates
        adapter_id against the recorded one (raising AdapterMismatchError on
        disagreement) and returns the existing entry without modification.
        """

        lock = self._acquire_lock()
        try:
            entries = self._read_index()
            if name in entries:
                rel = entries[name]
                env_dir = (self.root / rel).resolve() if (self.root / rel).exists() else self.root / rel
                meta_path = env_dir / "metadata.json"
                meta = Metadata.from_dict(json.loads(meta_path.read_text(encoding="utf-8")))
                if meta.adapter_id != adapter_id:
                    raise _errors.AdapterMismatchError(
                        f"env {name!r} was created with adapter {meta.adapter_id!r} "
                        f"but {adapter_id!r} was requested",
                        name=name,
                        expected_adapter_id=meta.adapter_id,
                        actual_adapter_id=adapter_id,
                    )
                return env_dir, meta, False
            slug = slug_for(name)
            # Avoid slug collision (vanishingly unlikely with sha256, but be defensive).
            existing_slugs = {Path(p).name for p in entries.values()}
            attempt = slug
            i = 0
            while attempt in existing_slugs:
                i += 1
                attempt = f"{slug}-{i}"
            slug = attempt
            rel_dir = f"envs/{slug}"
            env_dir = (self.root / rel_dir).absolute()
            (env_dir / "profile").mkdir(parents=True, exist_ok=True)
            now = _dt.datetime.now(_dt.timezone.utc).isoformat()
            meta = Metadata(
                schema_version=REGISTRY_SCHEMA_VERSION,
                name=name,
                adapter_id=adapter_id,
                created_at=now,
                env_overrides={},
                credentials_loaded=False,
                credentials_loaded_at=None,
            )
            (env_dir / "metadata.json").write_text(
                json.dumps(meta.to_dict(), indent=2), encoding="utf-8"
            )
            entries[name] = rel_dir
            self._write_index(entries)
            return env_dir.resolve(), meta, True
        finally:
            self._release_lock(lock)

    def update_metadata(self, env_dir: Path, meta: Metadata) -> None:
        path = env_dir / "metadata.json"
        try:
            path.write_text(json.dumps(meta.to_dict(), indent=2), encoding="utf-8")
        except OSError as exc:
            raise _errors.RegistryUnavailableError(
                f"writing metadata: {exc}", path=str(path)
            ) from exc

    def remove(self, name: str) -> tuple[bool, Path | None, str | None]:
        """Returns ``(ok, env_dir, error_message)``.

        Removes the on-disk env dir and the index entry. If the on-disk
        removal fails, the index entry is still removed and ok=False with a
        message.
        """

        lock = self._acquire_lock()
        try:
            entries = self._read_index()
            if name not in entries:
                raise _errors.EnvironmentNotFoundError(
                    f"no environment named {name!r}", name=name, registry_root=str(self.root)
                )
            rel = entries.pop(name)
            env_dir = (self.root / rel).absolute()
            cleanup_err: str | None = None
            try:
                if env_dir.exists():
                    shutil.rmtree(env_dir)
            except OSError as exc:
                cleanup_err = str(exc)
            self._write_index(entries)
            return cleanup_err is None, env_dir, cleanup_err
        finally:
            self._release_lock(lock)
