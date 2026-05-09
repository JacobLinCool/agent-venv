"""Codex adapter (Layer 2)."""

from __future__ import annotations

from pathlib import Path

from ..spec import EnvironmentSpec
from ._credentials import read_codex_auth
from .base import AgentAdapter


class Codex(AgentAdapter):
    """Build an EnvironmentSpec for an isolated ``CODEX_HOME``."""

    id = "codex"
    cli_bin = "codex"
    config_env_var = "CODEX_HOME"

    def __init__(
        self,
        *,
        model: str | None = None,
        reasoning_effort: str | None = None,
        load_credentials: bool = True,
        extra_argv: list[str] | None = None,
    ) -> None:
        self.model = model
        self.reasoning_effort = reasoning_effort
        self.load_credentials = load_credentials
        self.extra_argv = list(extra_argv or [])

    def build_spec(self, *, load_credentials: bool | None = None) -> EnvironmentSpec:
        load = self.load_credentials if load_credentials is None else load_credentials
        credentials: dict[str, str] = {}
        file_modes: dict[str, int] = {}
        if load:
            credentials["auth.json"] = read_codex_auth()
            file_modes["auth.json"] = 0o600
        return EnvironmentSpec(
            adapter_id=self.id,
            env_overrides={self.config_env_var: "$EPHEMERAL_HOME"},
            credentials=credentials,
            file_modes=file_modes,
            prefix="agent-venv-codex-",
        )

    def build_argv(self, prompt: str, *, workspace: Path | None = None) -> list[str]:
        if self.model is None:
            raise ValueError("Codex.build_argv requires model= to be set")
        argv = [
            self.cli_bin,
            "exec",
            "--model",
            self.model,
        ]
        if workspace is not None:
            argv += ["--cd", str(workspace)]
        argv += [
            "--json",
            "--skip-git-repo-check",
            "--dangerously-bypass-approvals-and-sandbox",
        ]
        if self.reasoning_effort is not None:
            argv += ["-c", f'model_reasoning_effort="{self.reasoning_effort}"']
        argv += self.extra_argv
        return argv
