"""Conformance adapter for ``agent-venv``.

Speaks the protocol described in spec/conformance-protocol.md.
"""

from __future__ import annotations

import json
import os
import stat
import sys
import traceback
from pathlib import Path
from typing import Any

from .. import errors as _errors
from .._version import SPEC_VERSION, __version__
from ..environment import Environment
from ..spec import EnvironmentSpec


def _inspect(env: Environment) -> dict[str, Any]:
    files: list[str] = []
    file_modes: dict[str, int] = {}
    if env.path.exists():
        for p in sorted(env.path.rglob("*")):
            if p.is_file():
                rel = str(p.relative_to(env.path))
                files.append(rel)
                if hasattr(os, "stat") and os.name == "posix":
                    mode = stat.S_IMODE(p.stat().st_mode)
                    file_modes[rel] = mode
    return {
        "path": str(env.path),
        "exists": env.path.exists(),
        "env_overrides": dict(env.env_overrides),
        "files_present": files,
        "file_modes": file_modes,
    }


def _spec_from_wire(payload: dict[str, Any] | None) -> EnvironmentSpec:
    return EnvironmentSpec.from_dict(payload or {})


def _events(env: Environment | None) -> list[dict[str, Any]]:
    if env is None:
        return []
    return [e.to_dict() for e in env.events()]


def _error_response(case_id: str, kind: str, message: str) -> dict[str, Any]:
    return {
        "case_id": case_id,
        "ok": True,
        "error": {"kind": kind, "message": message},
        "events": [],
    }


def _handle(req: dict[str, Any]) -> dict[str, Any]:
    case_id = str(req.get("case_id", ""))
    op = req.get("op")
    try:
        if op == "ephemeral_lifecycle":
            return _op_ephemeral_lifecycle(case_id, req)
        if op == "persistent_create_attach_idempotent":
            return _op_create_attach_idempotent(case_id, req)
        if op == "persistent_attach_only":
            return _op_attach_only(case_id, req)
        if op == "persistent_attach_missing":
            return _op_attach_missing(case_id, req)
        if op == "persistent_list":
            return _op_list(case_id, req)
        if op == "persistent_destroy_by_name":
            return _op_destroy_by_name(case_id, req)
        if op == "persistent_attach_mismatch":
            return _op_attach_mismatch(case_id, req)
        return {
            "case_id": case_id,
            "ok": False,
            "error": {
                "kind": "InternalInvariantViolation",
                "message": f"unknown op {op!r}",
            },
        }
    except _errors.AgentVenvError as exc:
        return _error_response(case_id, exc.kind, exc.message)
    except Exception:
        return _error_response(case_id, "InternalInvariantViolation", traceback.format_exc())


# ---------------------------------------------------------------------------
# Ops
# ---------------------------------------------------------------------------


def _op_ephemeral_lifecycle(case_id: str, req: dict[str, Any]) -> dict[str, Any]:
    spec = _spec_from_wire(req.get("spec"))
    env: Environment | None = None
    err: _errors.AgentVenvError | None = None
    inspection: dict[str, Any] = {}
    after_destroy: dict[str, Any] = {}
    try:
        env = Environment.ephemeral(spec=spec)
        inspection = _inspect(env)
    except _errors.AgentVenvError as exc:
        err = exc
    finally:
        if env is not None:
            env.destroy()
            after_destroy = {"path_exists": env.path.exists()}
    response: dict[str, Any] = {
        "case_id": case_id,
        "ok": True,
        "events": _events(env),
        "inspection": inspection,
        "after_destroy": after_destroy,
    }
    if err is not None:
        response["error"] = {"kind": err.kind, "message": err.message}
    return response


def _op_create_attach_idempotent(case_id: str, req: dict[str, Any]) -> dict[str, Any]:
    name = str(req.get("name", ""))
    registry_root = Path(req.get("registry_root", ""))
    spec = _spec_from_wire(req.get("spec"))

    env1 = Environment.create_or_attach(name, spec, registry_root=registry_root)
    path1 = str(env1.path)
    env2 = Environment.create_or_attach(name, spec, registry_root=registry_root)
    path2 = str(env2.path)
    inspection2 = _inspect(env2)
    files_present = inspection2["files_present"]
    response = {
        "case_id": case_id,
        "ok": True,
        "events": _events(env1) + _events(env2),
        "paths": [path1, path2],
        "second_path_files_present": files_present,
    }
    return response


def _op_attach_only(case_id: str, req: dict[str, Any]) -> dict[str, Any]:
    name = str(req.get("name", ""))
    registry_root = Path(req.get("registry_root", ""))
    env = Environment.attach(name, registry_root=registry_root)
    inspection = _inspect(env)
    return {
        "case_id": case_id,
        "ok": True,
        "events": _events(env),
        "path": str(env.path),
        "files_present": inspection["files_present"],
    }


def _op_attach_missing(case_id: str, req: dict[str, Any]) -> dict[str, Any]:
    name = str(req.get("name", ""))
    registry_root = Path(req.get("registry_root", ""))
    try:
        env = Environment.attach(name, registry_root=registry_root)
        return {
            "case_id": case_id,
            "ok": True,
            "events": _events(env),
            "error": {"kind": "InternalInvariantViolation", "message": "attach unexpectedly succeeded"},
        }
    except _errors.AgentVenvError as exc:
        return _error_response(case_id, exc.kind, exc.message)


def _op_list(case_id: str, req: dict[str, Any]) -> dict[str, Any]:
    names = req.get("names") or []
    registry_root = Path(req.get("registry_root", ""))
    spec = _spec_from_wire(req.get("spec"))
    for name in names:
        Environment.create_or_attach(str(name), spec, registry_root=registry_root)
    listed = Environment.list(registry_root=registry_root)
    return {
        "case_id": case_id,
        "ok": True,
        "events": [],
        "names_listed": listed,
    }


def _op_destroy_by_name(case_id: str, req: dict[str, Any]) -> dict[str, Any]:
    name = str(req.get("name", ""))
    registry_root = Path(req.get("registry_root", ""))
    spec = _spec_from_wire(req.get("spec"))
    env = Environment.create_or_attach(name, spec, registry_root=registry_root)
    created_path = env.path
    env.destroy()
    listed = Environment.list(registry_root=registry_root)
    return {
        "case_id": case_id,
        "ok": True,
        "events": _events(env),
        "created_path": str(created_path),
        "path_exists_after": created_path.exists(),
        "name_in_index_after": name in listed,
    }


def _op_attach_mismatch(case_id: str, req: dict[str, Any]) -> dict[str, Any]:
    name = str(req.get("name", ""))
    registry_root = Path(req.get("registry_root", ""))
    first_spec = _spec_from_wire(req.get("first_spec"))
    second_adapter_id = str(req.get("second_adapter_id"))
    Environment.create_or_attach(name, first_spec, registry_root=registry_root)
    second_spec = EnvironmentSpec(adapter_id=second_adapter_id)
    try:
        Environment.create_or_attach(name, second_spec, registry_root=registry_root)
        return _error_response(
            case_id, "InternalInvariantViolation", "expected AdapterMismatch but did not raise"
        )
    except _errors.AgentVenvError as exc:
        return _error_response(case_id, exc.kind, exc.message)


# ---------------------------------------------------------------------------
# Main loop
# ---------------------------------------------------------------------------


def main() -> int:
    sys.stdout.write(
        json.dumps(
            {
                "protocol": "agent-venv.conformance",
                "version": 2,
                "language": "python",
                "package_version": __version__,
                "spec_version": SPEC_VERSION,
            },
            separators=(",", ":"),
        )
        + "\n"
    )
    sys.stdout.flush()
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            request = json.loads(line)
        except json.JSONDecodeError as exc:
            sys.stdout.write(
                json.dumps(
                    {
                        "ok": False,
                        "error": {
                            "kind": "InternalInvariantViolation",
                            "message": f"bad request: {exc}",
                        },
                    }
                )
                + "\n"
            )
            sys.stdout.flush()
            continue
        try:
            response = _handle(request)
        except Exception:
            response = {
                "case_id": request.get("case_id", ""),
                "ok": False,
                "error": {
                    "kind": "InternalInvariantViolation",
                    "message": traceback.format_exc(),
                },
            }
        sys.stdout.write(json.dumps(response, separators=(",", ":")) + "\n")
        sys.stdout.flush()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
