"""AgentAdapter: thin convenience producing an EnvironmentSpec for a known CLI."""

from __future__ import annotations

import abc
import shutil
from pathlib import Path

from ..errors import AdapterUnavailableError
from ..spec import EnvironmentSpec


class AgentAdapter(abc.ABC):
    """Builds an :class:`EnvironmentSpec` for a particular CLI agent.

    Adapters are deliberately thin. They MUST NOT spawn processes — Layer 1
    doesn't run anything either. An adapter that needs more capability is a
    sign that capability should be lifted into Layer 1.
    """

    id: str
    cli_bin: str
    config_env_var: str

    def ensure_available(self) -> None:
        """Verify the adapter's CLI is on PATH. The library never calls this implicitly."""

        if shutil.which(self.cli_bin) is None:
            raise AdapterUnavailableError(
                f"{self.cli_bin!r} not found on PATH for adapter {self.id!r}",
                adapter_id=self.id,
                cli_bin=self.cli_bin,
            )

    @abc.abstractmethod
    def build_spec(self, *, load_credentials: bool | None = None) -> EnvironmentSpec:
        """Return the EnvironmentSpec describing the profile this adapter wants."""

    def build_argv(self, prompt: str, *, workspace: Path | None = None) -> list[str]:
        """Optional helper for callers who want a sensible default argv."""

        raise NotImplementedError
