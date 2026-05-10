"""Environment + AsyncEnvironment: the public Layer 1 entry points.

The library's job is to materialize a profile directory and hand the caller
the `path` and `env_overrides` they need to invoke `claude`/`codex`/etc.
The library does not run the agent.
"""

from __future__ import annotations

import datetime as _dt
import tempfile
from pathlib import Path
from typing import Any

from . import _profile
from . import errors as _errors
from .adapters.base import AgentAdapter
from .events import Event, EventLog, EventSink
from .registry import Registry, default_registry_root
from .spec import EnvironmentSpec


class _BaseEnvironment:
    """Shared logic between ``Environment`` (sync) and ``AsyncEnvironment``."""

    def __init__(
        self,
        *,
        path: Path,
        env_overrides: dict[str, str],
        spec: EnvironmentSpec,
        log: EventLog,
        kind: str,
        name: str | None,
        registry: Registry | None,
    ) -> None:
        self._path = path
        self._env_overrides = dict(env_overrides)
        self._spec = spec
        self._log = log
        self._kind = kind  # "ephemeral" | "persistent"
        self._name = name
        self._registry = registry
        self._destroyed = False

    @property
    def path(self) -> Path:
        return self._path

    @property
    def env_overrides(self) -> dict[str, str]:
        return dict(self._env_overrides)

    @property
    def adapter_id(self) -> str:
        return self._spec.adapter_id

    @property
    def name(self) -> str | None:
        return self._name

    @property
    def kind(self) -> str:
        return self._kind

    @property
    def is_persistent(self) -> bool:
        return self._kind == "persistent"

    def events(self) -> list[Event]:
        return self._log.all()

    def _do_destroy(self) -> bool:
        if self._destroyed:
            return True
        ok = True
        if self._kind == "persistent":
            assert self._registry is not None and self._name is not None
            try:
                cleanup_ok, env_dir, cleanup_err = self._registry.remove(self._name)
                ok = cleanup_ok
                self._log.emit("registry.written", path=str(self._registry._index_path))
                self._log.emit(
                    "env.destroyed",
                    path=str(env_dir or self._path),
                    ok=ok,
                )
                if not ok:
                    self._log.emit(
                        "error",
                        error_kind=_errors.CleanupFailedError.kind,
                        message=cleanup_err or "",
                    )
            except _errors.AgentVenvError as exc:
                # Already gone, etc.
                self._log.emit("error", error_kind=exc.kind, message=exc.message)
                ok = True  # already-gone counts as success
        else:
            ok = _profile.remove_dir(self._path)
            self._log.emit("env.destroyed", path=str(self._path), ok=ok)
            if not ok:
                self._log.emit(
                    "error",
                    error_kind=_errors.CleanupFailedError.kind,
                    message="rmtree failed",
                )
        self._destroyed = True
        return ok


# ---------------------------------------------------------------------------
# Sync Environment
# ---------------------------------------------------------------------------


class Environment(_BaseEnvironment):
    """Synchronous Layer 1 environment handle."""

    # ------------------------------------------------------------------
    # Ephemeral
    # ------------------------------------------------------------------

    @classmethod
    def ephemeral(
        cls,
        spec: EnvironmentSpec | None = None,
        *,
        adapter: AgentAdapter | None = None,
        on_event: EventSink | None = None,
    ) -> Environment:
        spec = _resolve_spec(spec, adapter)
        log = EventLog(on_event=on_event)
        try:
            path = Path(tempfile.mkdtemp(prefix=spec.prefix)).resolve()
        except OSError as exc:
            raise _errors.ProfileSetupFailedError(
                f"creating ephemeral profile dir: {exc}", reason="mkdtemp_failed"
            ) from exc
        log.emit(
            "env.created",
            name=None,
            lifetime="ephemeral",
            adapter_id=spec.adapter_id,
            path=str(path),
        )
        try:
            env_overrides = _profile.materialize(path, spec, log)
        except _errors.AgentVenvError as exc:
            log.emit("error", error_kind=exc.kind, message=exc.message)
            _profile.remove_dir(path)
            raise
        return cls(
            path=path,
            env_overrides=env_overrides,
            spec=spec,
            log=log,
            kind="ephemeral",
            name=None,
            registry=None,
        )

    # ------------------------------------------------------------------
    # Persistent
    # ------------------------------------------------------------------

    @classmethod
    def create_or_attach(
        cls,
        name: str,
        spec: EnvironmentSpec | None = None,
        *,
        adapter: AgentAdapter | None = None,
        registry_root: Path | None = None,
        on_event: EventSink | None = None,
    ) -> Environment:
        spec = _resolve_spec(spec, adapter)
        log = EventLog(on_event=on_event)
        registry = Registry(registry_root or default_registry_root())
        env_dir, meta, created = registry.reserve_or_get(name, spec.adapter_id)
        profile_dir = (env_dir / "profile").resolve()
        if created:
            log.emit(
                "env.created",
                name=name,
                lifetime="persistent",
                adapter_id=spec.adapter_id,
                path=str(profile_dir),
            )
            env_overrides = _profile.materialize(profile_dir, spec, log)
            now = _dt.datetime.now(_dt.timezone.utc).isoformat()
            meta.env_overrides = env_overrides
            meta.credentials_loaded = bool(spec.credentials)
            if meta.credentials_loaded:
                meta.credentials_loaded_at = now
            registry.update_metadata(env_dir, meta)
            log.emit("registry.written", path=str(env_dir / "metadata.json"))
        else:
            # Attach: no rewrite. Use recorded env_overrides if non-empty;
            # otherwise recompute from the spec we were given (callers expect
            # env_overrides to reflect $EPHEMERAL_HOME placeholder).
            log.emit(
                "env.attached",
                name=name,
                adapter_id=spec.adapter_id,
                path=str(profile_dir),
            )
            log.emit("registry.read", path=str(env_dir / "metadata.json"))
            env_overrides = (
                dict(meta.env_overrides)
                if meta.env_overrides
                else _profile.materialize(profile_dir, spec, log, skip_seed_if_exists=True)
            )
        return cls(
            path=profile_dir,
            env_overrides=env_overrides,
            spec=spec,
            log=log,
            kind="persistent",
            name=name,
            registry=registry,
        )

    @classmethod
    def attach(
        cls,
        name: str,
        *,
        adapter: AgentAdapter | None = None,
        registry_root: Path | None = None,
        on_event: EventSink | None = None,
    ) -> Environment:
        log = EventLog(on_event=on_event)
        registry = Registry(registry_root or default_registry_root())
        result = registry.lookup(name)
        if result is None:
            raise _errors.EnvironmentNotFoundError(
                f"no environment named {name!r}", name=name, registry_root=str(registry.root)
            )
        env_dir, meta = result
        if adapter is not None and adapter.id != meta.adapter_id:
            raise _errors.AdapterMismatchError(
                f"env {name!r} was created with adapter {meta.adapter_id!r} "
                f"but {adapter.id!r} was passed to attach",
                name=name,
                expected_adapter_id=meta.adapter_id,
                actual_adapter_id=adapter.id,
            )
        profile_dir = (env_dir / "profile").resolve()
        spec = EnvironmentSpec(adapter_id=meta.adapter_id, env_overrides=dict(meta.env_overrides))
        log.emit(
            "env.attached",
            name=name,
            adapter_id=meta.adapter_id,
            path=str(profile_dir),
        )
        log.emit("registry.read", path=str(env_dir / "metadata.json"))
        return cls(
            path=profile_dir,
            env_overrides=dict(meta.env_overrides),
            spec=spec,
            log=log,
            kind="persistent",
            name=name,
            registry=registry,
        )

    @classmethod
    def list(cls, *, registry_root: Path | None = None) -> list[str]:
        return Registry(registry_root or default_registry_root()).list_names()

    @classmethod
    def destroy_by_name(cls, name: str, *, registry_root: Path | None = None) -> bool:
        registry = Registry(registry_root or default_registry_root())
        ok, _, err = registry.remove(name)
        if not ok and err:
            raise _errors.CleanupFailedError(err, path=str(registry.root), os_error=err)
        return ok

    # ------------------------------------------------------------------
    # Instance API
    # ------------------------------------------------------------------

    def __enter__(self) -> Environment:
        return self

    def __exit__(self, exc_type: Any, exc: Any, tb: Any) -> None:
        if self._kind == "ephemeral":
            self.destroy()

    def destroy(self) -> bool:
        return self._do_destroy()

    def refresh_credentials(self, spec: EnvironmentSpec | None = None) -> None:
        if self._destroyed:
            raise _errors.InternalInvariantViolationError(
                "refresh_credentials() on destroyed environment"
            )
        spec = spec or self._spec
        if not spec.credentials:
            return
        try:
            count, _ = _profile._write_files(  # type: ignore[attr-defined]
                self._path, spec.credentials, spec.file_modes, default_mode=0o600
            )
        except OSError as exc:
            raise _errors.ProfileSetupFailedError(
                f"refreshing credentials: {exc}", reason="write_failed"
            ) from exc
        self._log.emit("credentials.refreshed", file_count=count)


# ---------------------------------------------------------------------------
# Async Environment
# ---------------------------------------------------------------------------


class AsyncEnvironment(_BaseEnvironment):
    """asyncio twin of :class:`Environment`. Use ``async with``.

    All filesystem work happens synchronously in v0; the async wrappers exist
    for ergonomic interop with async callers (so they can ``await``
    creation alongside other awaits).
    """

    @classmethod
    async def ephemeral(
        cls,
        spec: EnvironmentSpec | None = None,
        *,
        adapter: AgentAdapter | None = None,
        on_event: EventSink | None = None,
    ) -> AsyncEnvironment:
        sync = Environment.ephemeral(spec, adapter=adapter, on_event=on_event)
        return cls._from_sync(sync)

    @classmethod
    async def create_or_attach(
        cls,
        name: str,
        spec: EnvironmentSpec | None = None,
        *,
        adapter: AgentAdapter | None = None,
        registry_root: Path | None = None,
        on_event: EventSink | None = None,
    ) -> AsyncEnvironment:
        sync = Environment.create_or_attach(
            name, spec, adapter=adapter, registry_root=registry_root, on_event=on_event
        )
        return cls._from_sync(sync)

    @classmethod
    async def attach(
        cls,
        name: str,
        *,
        adapter: AgentAdapter | None = None,
        registry_root: Path | None = None,
        on_event: EventSink | None = None,
    ) -> AsyncEnvironment:
        sync = Environment.attach(
            name, adapter=adapter, registry_root=registry_root, on_event=on_event
        )
        return cls._from_sync(sync)

    @classmethod
    def _from_sync(cls, sync: Environment) -> AsyncEnvironment:
        return cls(
            path=sync.path,
            env_overrides=sync.env_overrides,
            spec=sync._spec,
            log=sync._log,
            kind=sync.kind,
            name=sync.name,
            registry=sync._registry,
        )

    async def __aenter__(self) -> AsyncEnvironment:
        return self

    async def __aexit__(self, exc_type: Any, exc: Any, tb: Any) -> None:
        if self._kind == "ephemeral":
            await self.destroy()

    async def destroy(self) -> bool:
        return self._do_destroy()


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _resolve_spec(spec: EnvironmentSpec | None, adapter: AgentAdapter | None) -> EnvironmentSpec:
    if spec is not None and adapter is not None:
        raise _errors.InvalidEnvironmentSpecError(
            "pass either spec= or adapter=, not both", field="spec/adapter"
        )
    if adapter is not None:
        return adapter.build_spec()
    if spec is None:
        return EnvironmentSpec()
    return spec
