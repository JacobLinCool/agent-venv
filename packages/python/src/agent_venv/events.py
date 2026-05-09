"""Event log for environment lifecycles."""

from __future__ import annotations

import dataclasses
import time
from typing import Any, Literal, Protocol


EventKind = Literal[
    "env.created",
    "env.attached",
    "profile.materialized",
    "credentials.copied",
    "credentials.refreshed",
    "env.destroyed",
    "registry.read",
    "registry.written",
    "error",
]


@dataclasses.dataclass(slots=True)
class Event:
    ts_ms: int
    kind: EventKind
    data: dict[str, Any] = dataclasses.field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        return {"ts_ms": self.ts_ms, "kind": self.kind, **self.data}


class EventSink(Protocol):
    def __call__(self, event: Event) -> None: ...


class EventLog:
    def __init__(self, on_event: EventSink | None = None) -> None:
        self._origin = time.monotonic()
        self._events: list[Event] = []
        self._on_event = on_event

    def emit(self, kind: EventKind, /, **data: Any) -> Event:
        ts_ms = int((time.monotonic() - self._origin) * 1000)
        event = Event(ts_ms=ts_ms, kind=kind, data=data)
        self._events.append(event)
        if self._on_event is not None:
            try:
                self._on_event(event)
            except Exception:
                pass
        return event

    def all(self) -> list[Event]:
        return list(self._events)
