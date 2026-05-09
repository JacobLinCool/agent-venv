"""Error taxonomy for agent-venv.

Every error subclasses :class:`AgentVenvError` and carries a ``kind``
string matching one of the entries in spec/errors.md.
"""

from __future__ import annotations

from typing import Any


class AgentVenvError(Exception):
    """Base for every error raised by this library."""

    kind: str = "InternalInvariantViolation"

    def __init__(self, message: str = "", **payload: Any) -> None:
        super().__init__(message or self.kind)
        self.message = message or self.kind
        self.payload: dict[str, Any] = payload

    def to_dict(self) -> dict[str, Any]:
        return {"kind": self.kind, "message": self.message, **self.payload}


class EnvironmentNotFoundError(AgentVenvError):
    kind = "EnvironmentNotFound"


class EnvironmentAlreadyExistsError(AgentVenvError):
    kind = "EnvironmentAlreadyExists"


class AdapterMismatchError(AgentVenvError):
    kind = "AdapterMismatch"


class ProfileSetupFailedError(AgentVenvError):
    kind = "ProfileSetupFailed"


class RegistryUnavailableError(AgentVenvError):
    kind = "RegistryUnavailable"


class CredentialsMissingError(AgentVenvError):
    kind = "CredentialsMissing"


class AdapterUnavailableError(AgentVenvError):
    kind = "AdapterUnavailable"


class CleanupFailedError(AgentVenvError):
    kind = "CleanupFailed"


class InvalidEnvironmentSpecError(AgentVenvError):
    kind = "InvalidEnvironmentSpec"


class InternalInvariantViolationError(AgentVenvError):
    kind = "InternalInvariantViolation"


_BY_KIND: dict[str, type[AgentVenvError]] = {
    cls.kind: cls
    for cls in (
        EnvironmentNotFoundError,
        EnvironmentAlreadyExistsError,
        AdapterMismatchError,
        ProfileSetupFailedError,
        RegistryUnavailableError,
        CredentialsMissingError,
        AdapterUnavailableError,
        CleanupFailedError,
        InvalidEnvironmentSpecError,
        InternalInvariantViolationError,
    )
}


def for_kind(kind: str) -> type[AgentVenvError]:
    return _BY_KIND.get(kind, InternalInvariantViolationError)


__all__ = [
    "AdapterMismatchError",
    "AdapterUnavailableError",
    "CleanupFailedError",
    "CredentialsMissingError",
    "EnvironmentAlreadyExistsError",
    "EnvironmentNotFoundError",
    "InternalInvariantViolationError",
    "InvalidEnvironmentSpecError",
    "AgentVenvError",
    "ProfileSetupFailedError",
    "RegistryUnavailableError",
    "for_kind",
]
