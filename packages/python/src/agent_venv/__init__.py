"""agent-venv: virtualenv-style profile isolation for coding-agent CLIs.

The library creates an isolated config directory for `claude`, `codex`, etc.,
optionally copies the user's host credentials into it, and returns the path +
env vars the caller should set when invoking the agent. The library never
runs the agent itself.

See spec/ at the repo root for the cross-language contract.
"""

from . import adapters, errors
from ._version import __version__
from .adapters.claude_code import ClaudeCode
from .adapters.codex import Codex
from .environment import AsyncEnvironment, Environment
from .events import Event, EventSink
from .registry import default_registry_root
from .spec import EnvironmentSpec

__all__ = [
    "AsyncEnvironment",
    "ClaudeCode",
    "Codex",
    "Environment",
    "EnvironmentSpec",
    "Event",
    "EventSink",
    "__version__",
    "adapters",
    "default_registry_root",
    "errors",
]
