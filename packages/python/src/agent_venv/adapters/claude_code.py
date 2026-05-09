"""Claude Code adapter (Layer 2)."""

from __future__ import annotations

from pathlib import Path

from ..spec import EnvironmentSpec
from ._credentials import read_claude_credentials
from .base import AgentAdapter


class ClaudeCode(AgentAdapter):
    """Build an EnvironmentSpec for an isolated ``CLAUDE_CONFIG_DIR``.

    The user's host OAuth credentials are (by default) read once from the
    macOS Keychain or ``~/.claude/.credentials.json`` and recorded in the
    spec's ``credentials`` map. The library writes them into the new
    profile dir at materialization time. The originals are not modified.
    """

    id = "claude-code"
    cli_bin = "claude"
    config_env_var = "CLAUDE_CONFIG_DIR"

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
        seed_files: dict[str, str] = {}
        credentials: dict[str, str] = {}
        file_modes: dict[str, int] = {}
        if load:
            credentials[".credentials.json"] = read_claude_credentials()
            file_modes[".credentials.json"] = 0o600
        else:
            # Onboarding marker so the CLI doesn't pop a setup flow.
            seed_files[".claude.json"] = '{"hasCompletedOnboarding": true}'
        return EnvironmentSpec(
            adapter_id=self.id,
            env_overrides={self.config_env_var: "$EPHEMERAL_HOME"},
            seed_files=seed_files,
            credentials=credentials,
            file_modes=file_modes,
            prefix="agent-venv-claude-",
        )

    def build_argv(self, prompt: str, *, workspace: Path | None = None) -> list[str]:
        if self.model is None:
            raise ValueError("ClaudeCode.build_argv requires model= to be set")
        argv = [
            self.cli_bin,
            "--print",
            "--model",
            self.model,
            "--output-format",
            "stream-json",
            "--verbose",
            "--dangerously-skip-permissions",
        ]
        if self.reasoning_effort is not None:
            argv += ["--effort", self.reasoning_effort]
        argv += self.extra_argv
        return argv
