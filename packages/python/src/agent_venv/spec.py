"""EnvironmentSpec: declarative description of a profile to materialize."""

from __future__ import annotations

import dataclasses
from typing import Any


@dataclasses.dataclass(slots=True)
class EnvironmentSpec:
    """Inputs to ``Environment.ephemeral`` / ``Environment.create_or_attach``.

    See spec/environment-spec.schema.json for the language-neutral contract.
    """

    adapter_id: str = "generic"
    env_overrides: dict[str, str] = dataclasses.field(default_factory=dict)
    seed_files: dict[str, str] = dataclasses.field(default_factory=dict)
    file_modes: dict[str, int] = dataclasses.field(default_factory=dict)
    credentials: dict[str, str] = dataclasses.field(default_factory=dict)
    prefix: str = "agent-venv-"

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> EnvironmentSpec:
        return cls(
            adapter_id=str(data.get("adapter_id", "generic")),
            env_overrides=dict(data.get("env_overrides") or {}),
            seed_files=dict(data.get("seed_files") or {}),
            file_modes={k: int(v) for k, v in (data.get("file_modes") or {}).items()},
            credentials=dict(data.get("credentials") or {}),
            prefix=str(data.get("prefix", "agent-venv-")),
        )

    def to_dict(self) -> dict[str, Any]:
        return {
            "adapter_id": self.adapter_id,
            "env_overrides": dict(self.env_overrides),
            "seed_files": dict(self.seed_files),
            "file_modes": dict(self.file_modes),
            "credentials": dict(self.credentials),
            "prefix": self.prefix,
        }
