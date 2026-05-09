"""Conformance runner.

Drives one or more language adapter CLIs by speaking NDJSON over stdin/stdout
(see spec/conformance-protocol.md). For each case, sends the request to every
adapter, validates each response against the case's expect block, and
optionally cross-checks consistency between adapters.
"""

from __future__ import annotations

import dataclasses
import json
import os
import shutil
import subprocess
import sys
import tempfile
import time
import uuid
from collections import Counter
from pathlib import Path
from typing import Any


# ---------------------------------------------------------------------------
# Data types
# ---------------------------------------------------------------------------


@dataclasses.dataclass
class AdapterCLI:
    name: str
    command: list[str]


@dataclasses.dataclass
class Case:
    case_id: str
    request: dict[str, Any]
    expect: dict[str, Any]
    source_path: Path


@dataclasses.dataclass
class CaseOutcome:
    case_id: str
    adapter: str
    response: dict[str, Any] | None
    passed: bool
    failures: list[str]
    skipped: bool = False
    skip_reason: str | None = None


@dataclasses.dataclass
class DifferentialOutcome:
    case_id: str
    passed: bool
    failures: list[str]


@dataclasses.dataclass
class Report:
    per_adapter: list[CaseOutcome]
    differential: list[DifferentialOutcome]
    adapters: list[str]

    @property
    def passed(self) -> bool:
        return all(o.passed or o.skipped for o in self.per_adapter) and all(
            d.passed for d in self.differential
        )


# ---------------------------------------------------------------------------
# Case loading
# ---------------------------------------------------------------------------


def load_cases(root: Path) -> list[Case]:
    cases: list[Case] = []
    for path in sorted(root.rglob("*.json")):
        try:
            data = json.loads(path.read_text(encoding="utf-8"))
        except json.JSONDecodeError as exc:
            raise SystemExit(f"invalid JSON in {path}: {exc}") from exc
        case_id = data.get("case_id") or path.stem
        request = data.get("request")
        expect = data.get("expect", {})
        if not isinstance(request, dict):
            raise SystemExit(f"{path}: missing or non-object 'request'")
        cases.append(Case(case_id=case_id, request=request, expect=expect, source_path=path))
    return cases


# ---------------------------------------------------------------------------
# Adapter session
# ---------------------------------------------------------------------------


class AdapterSession:
    """Persistent NDJSON session with one adapter CLI."""

    def __init__(self, adapter: AdapterCLI):
        self.adapter = adapter
        if not adapter.command:
            raise ValueError(f"adapter {adapter.name!r} has empty command")
        bin_name = adapter.command[0]
        if "/" not in bin_name and shutil.which(bin_name) is None:
            print(
                f"warning: adapter {adapter.name!r} bin {bin_name!r} not on PATH",
                file=sys.stderr,
            )
        self.proc = subprocess.Popen(
            adapter.command,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
            env=dict(os.environ),
        )
        banner_line = self._read_line(timeout_s=10.0)
        try:
            banner = json.loads(banner_line)
        except json.JSONDecodeError as exc:
            raise RuntimeError(
                f"adapter {adapter.name!r} did not emit a valid banner. got: {banner_line!r}"
            ) from exc
        if banner.get("protocol") != "agent-venv.conformance":
            raise RuntimeError(
                f"adapter {adapter.name!r} banner has wrong protocol: {banner}"
            )
        self.banner = banner

    def _read_line(self, timeout_s: float) -> str:
        deadline = time.monotonic() + timeout_s
        assert self.proc.stdout is not None
        while time.monotonic() < deadline:
            if self.proc.poll() is not None:
                stderr = self._drain_stderr()
                raise RuntimeError(
                    f"adapter {self.adapter.name!r} exited (code={self.proc.returncode}). stderr:\n{stderr}"
                )
            line = self.proc.stdout.readline()
            if line:
                return line.strip()
        raise RuntimeError(
            f"adapter {self.adapter.name!r} did not respond within {timeout_s}s"
        )

    def _drain_stderr(self) -> str:
        try:
            assert self.proc.stderr is not None
            return self.proc.stderr.read() or ""
        except Exception:
            return "<stderr unreadable>"

    def request(self, payload: dict[str, Any], timeout_s: float = 60.0) -> dict[str, Any]:
        line = json.dumps(payload, separators=(",", ":")) + "\n"
        assert self.proc.stdin is not None
        try:
            self.proc.stdin.write(line)
            self.proc.stdin.flush()
        except BrokenPipeError as exc:
            stderr = self._drain_stderr()
            raise RuntimeError(
                f"adapter {self.adapter.name!r} stdin closed. stderr:\n{stderr}"
            ) from exc
        response_line = self._read_line(timeout_s=timeout_s)
        try:
            return json.loads(response_line)
        except json.JSONDecodeError as exc:
            raise RuntimeError(
                f"adapter {self.adapter.name!r} returned non-JSON: {response_line!r}"
            ) from exc

    def close(self) -> tuple[int | None, str]:
        if self.proc.stdin is not None:
            try:
                self.proc.stdin.close()
            except Exception:
                pass
        try:
            self.proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            self.proc.kill()
            self.proc.wait()
        stderr = self._drain_stderr()
        return self.proc.returncode, stderr


# ---------------------------------------------------------------------------
# Per-case driver: most cases pass the request through to each adapter
# unchanged. cross_language_attach is special: stage 1 picks one adapter to
# create the env, stage 2 sends an `attach_only` request to every adapter.
# ---------------------------------------------------------------------------


class Runner:
    def __init__(
        self,
        adapters: list[AdapterCLI],
        cases: list[Case],
        differential: bool = True,
    ):
        self.adapters = adapters
        self.cases = cases
        self.differential = differential

    def run(self, verbose: int = 0) -> Report:
        per_adapter: list[CaseOutcome] = []
        # case_id -> adapter -> response
        responses: dict[str, dict[str, dict[str, Any]]] = {}

        sessions: dict[str, AdapterSession] = {}
        try:
            for adapter in self.adapters:
                try:
                    sessions[adapter.name] = AdapterSession(adapter)
                except Exception as exc:
                    for case in self.cases:
                        per_adapter.append(
                            CaseOutcome(
                                case_id=case.case_id,
                                adapter=adapter.name,
                                response=None,
                                passed=False,
                                failures=[f"adapter startup failed: {exc}"],
                            )
                        )

            available_adapters = [a.name for a in self.adapters if a.name in sessions]

            for case in self.cases:
                op = case.request.get("op")
                if op == "cross_language_attach":
                    self._run_cross_language(
                        case, sessions, available_adapters, per_adapter, responses, verbose
                    )
                else:
                    self._run_simple(
                        case, sessions, available_adapters, per_adapter, responses, verbose
                    )
        finally:
            for s in sessions.values():
                s.close()

        differential_results: list[DifferentialOutcome] = []
        if self.differential and len(self.adapters) >= 2:
            for case in self.cases:
                got = responses.get(case.case_id, {})
                if len(got) < 2:
                    continue
                failures = check_differential(got, case)
                differential_results.append(
                    DifferentialOutcome(case_id=case.case_id, passed=not failures, failures=failures)
                )

        return Report(
            per_adapter=per_adapter,
            differential=differential_results,
            adapters=[a.name for a in self.adapters],
        )

    def _run_simple(
        self,
        case: Case,
        sessions: dict[str, AdapterSession],
        available: list[str],
        per_adapter: list[CaseOutcome],
        responses: dict[str, dict[str, dict[str, Any]]],
        verbose: int,
    ) -> None:
        op = case.request.get("op")
        # For ops that need a registry_root, mint a fresh one per adapter so they don't
        # interfere with each other.
        for name in available:
            if verbose:
                print(f"[{name}] {case.case_id}", file=sys.stderr)
            payload = dict(case.request)
            payload.setdefault("case_id", case.case_id)
            registry_root = None
            if op and op.startswith("persistent"):
                registry_root = tempfile.mkdtemp(prefix=f"iso-conf-{case.case_id}-{name}-")
                payload["registry_root"] = registry_root
            try:
                response = sessions[name].request(payload)
            except Exception as exc:
                per_adapter.append(
                    CaseOutcome(
                        case_id=case.case_id,
                        adapter=name,
                        response=None,
                        passed=False,
                        failures=[f"transport: {exc}"],
                    )
                )
                if registry_root:
                    shutil.rmtree(registry_root, ignore_errors=True)
                continue
            failures = check_expect(case, response)
            per_adapter.append(
                CaseOutcome(
                    case_id=case.case_id,
                    adapter=name,
                    response=response,
                    passed=not failures,
                    failures=failures,
                )
            )
            responses.setdefault(case.case_id, {})[name] = response
            if registry_root:
                shutil.rmtree(registry_root, ignore_errors=True)

    def _run_cross_language(
        self,
        case: Case,
        sessions: dict[str, AdapterSession],
        available: list[str],
        per_adapter: list[CaseOutcome],
        responses: dict[str, dict[str, dict[str, Any]]],
        verbose: int,
    ) -> None:
        if len(available) < 2:
            for name in available:
                per_adapter.append(
                    CaseOutcome(
                        case_id=case.case_id,
                        adapter=name,
                        response={"ok": True, "skipped": True},
                        passed=True,
                        failures=[],
                        skipped=True,
                        skip_reason="cross_language requires >= 2 adapters",
                    )
                )
            return

        registry_root = tempfile.mkdtemp(prefix=f"iso-conf-{case.case_id}-")
        try:
            creator = available[0]
            attachers = available[1:]
            spec = case.request.get("spec") or {}
            name = case.request.get("name") or f"shared-{uuid.uuid4().hex[:8]}"

            create_payload: dict[str, Any] = {
                "case_id": case.case_id,
                "op": "persistent_create_attach_idempotent",
                "name": name,
                "registry_root": registry_root,
                "spec": spec,
            }
            try:
                creator_resp = sessions[creator].request(create_payload)
            except Exception as exc:
                per_adapter.append(
                    CaseOutcome(
                        case_id=case.case_id,
                        adapter=creator,
                        response=None,
                        passed=False,
                        failures=[f"transport (create): {exc}"],
                    )
                )
                return
            creator_paths = creator_resp.get("paths", [])
            creator_path = creator_paths[0] if creator_paths else None
            # Re-frame as if the creator did "cross_language_attach" with the canonical path.
            creator_synth = {
                **creator_resp,
                "path": creator_path,
                "second_path_files_present": creator_resp.get("second_path_files_present", []),
            }
            failures = check_expect(case, creator_synth)
            # Augment: cross_language_paths_equal is checked across all adapters below.
            per_adapter.append(
                CaseOutcome(
                    case_id=case.case_id,
                    adapter=creator,
                    response=creator_synth,
                    passed=not failures,
                    failures=failures,
                )
            )
            responses.setdefault(case.case_id, {})[creator] = creator_synth

            for name_a in attachers:
                attach_payload: dict[str, Any] = {
                    "case_id": case.case_id,
                    "op": "persistent_attach_only",
                    "name": name,
                    "registry_root": registry_root,
                }
                try:
                    resp = sessions[name_a].request(attach_payload)
                except Exception as exc:
                    per_adapter.append(
                        CaseOutcome(
                            case_id=case.case_id,
                            adapter=name_a,
                            response=None,
                            passed=False,
                            failures=[f"transport (attach): {exc}"],
                        )
                    )
                    continue
                # Synthesize fields the case expects
                attach_path = resp.get("path")
                files_present = resp.get("files_present", [])
                synth = {
                    **resp,
                    "second_path_files_present": files_present,
                    "path": attach_path,
                }
                local_failures: list[str] = []
                if not synth.get("ok"):
                    local_failures.append(f"attach not ok: {synth.get('error')}")
                if attach_path != creator_path:
                    local_failures.append(
                        f"path mismatch: creator={creator_path!r} this={attach_path!r}"
                    )
                expected_files = case.expect.get("second_path_files_present", [])
                missing = [f for f in expected_files if f not in files_present]
                if missing:
                    local_failures.append(f"missing files after attach: {missing}")
                per_adapter.append(
                    CaseOutcome(
                        case_id=case.case_id,
                        adapter=name_a,
                        response=synth,
                        passed=not local_failures,
                        failures=local_failures,
                    )
                )
                responses.setdefault(case.case_id, {})[name_a] = synth
        finally:
            shutil.rmtree(registry_root, ignore_errors=True)


# ---------------------------------------------------------------------------
# Per-adapter assertions
# ---------------------------------------------------------------------------


def check_expect(case: Case, response: dict[str, Any]) -> list[str]:
    expect = case.expect
    failures: list[str] = []

    if response.get("skipped"):
        return []

    if "ok" in expect and bool(response.get("ok")) != bool(expect["ok"]):
        failures.append(f"ok mismatch: expect={expect['ok']} got={response.get('ok')}")

    if expect.get("error_absent"):
        if response.get("error"):
            failures.append(f"expected no error but got {response['error']!r}")

    if "error_kind" in expect:
        err = response.get("error")
        if not err:
            failures.append(f"expected error.kind={expect['error_kind']!r} but no error in response")
        elif err.get("kind") != expect["error_kind"]:
            failures.append(
                f"error.kind mismatch: expect={expect['error_kind']!r} got={err.get('kind')!r}"
            )

    inspection = response.get("inspection") or {}
    if "inspection_path_exists" in expect:
        if bool(inspection.get("exists")) != bool(expect["inspection_path_exists"]):
            failures.append(
                f"inspection.exists mismatch: expect={expect['inspection_path_exists']!r} "
                f"got={inspection.get('exists')!r}"
            )
    if "inspection_files_present" in expect:
        actual = inspection.get("files_present", [])
        for f in expect["inspection_files_present"]:
            if f not in actual:
                failures.append(f"inspection missing file {f!r}; got {actual}")

    if "env_overrides_keys" in expect:
        eo = inspection.get("env_overrides", {}) or response.get("env_overrides", {})
        actual = sorted(eo.keys())
        want = sorted(expect["env_overrides_keys"])
        if actual != want:
            failures.append(
                f"env_overrides keys mismatch: expect={want} got={actual}"
            )
    if "env_overrides_excludes" in expect:
        eo = inspection.get("env_overrides", {}) or response.get("env_overrides", {})
        for k in expect["env_overrides_excludes"]:
            if k in eo:
                failures.append(f"env_overrides should NOT contain key {k!r}")
    if expect.get("env_overrides_foo_is_path"):
        eo = inspection.get("env_overrides", {}) or response.get("env_overrides", {})
        path = inspection.get("path")
        if path and eo.get("FOO") != path:
            failures.append(
                f"env_overrides.FOO should equal inspection.path; "
                f"FOO={eo.get('FOO')!r} path={path!r}"
            )

    if "credentials_mode_unix" in expect and sys.platform != "win32":
        modes = inspection.get("file_modes", {}) or {}
        cred = next((k for k in modes if k == ".credentials.json" or k == "auth.json"), None)
        if cred is None:
            failures.append(f"no credential mode reported; got {modes!r}")
        elif modes[cred] != expect["credentials_mode_unix"]:
            failures.append(
                f"credentials mode mismatch on {cred}: expect={expect['credentials_mode_unix']} "
                f"got={modes[cred]}"
            )
    if "file_mode_data_txt_unix" in expect and sys.platform != "win32":
        modes = inspection.get("file_modes", {}) or {}
        if modes.get("data.txt") != expect["file_mode_data_txt_unix"]:
            failures.append(
                f"data.txt mode mismatch: expect={expect['file_mode_data_txt_unix']} got={modes.get('data.txt')}"
            )

    if "after_destroy_path_exists" in expect:
        ad = response.get("after_destroy") or {}
        if bool(ad.get("path_exists")) != bool(expect["after_destroy_path_exists"]):
            failures.append(
                f"after_destroy.path_exists mismatch: expect={expect['after_destroy_path_exists']!r} "
                f"got={ad.get('path_exists')!r}"
            )

    if "paths_equal" in expect:
        paths = response.get("paths", [])
        if len(paths) < 2:
            failures.append(f"expected 2 paths, got {len(paths)}")
        elif (paths[0] == paths[1]) != bool(expect["paths_equal"]):
            failures.append(f"paths_equal mismatch: paths={paths}")

    if "second_path_files_present" in expect:
        actual = response.get("second_path_files_present", [])
        for f in expect["second_path_files_present"]:
            if f not in actual:
                failures.append(f"second path missing {f!r}; got {actual}")

    if "names_listed_sorted" in expect:
        actual = response.get("names_listed", [])
        if sorted(actual) != sorted(expect["names_listed_sorted"]):
            failures.append(
                f"names_listed mismatch: expect={expect['names_listed_sorted']!r} got={actual!r}"
            )

    if "path_exists_after" in expect:
        if bool(response.get("path_exists_after")) != bool(expect["path_exists_after"]):
            failures.append(
                f"path_exists_after mismatch: expect={expect['path_exists_after']!r} "
                f"got={response.get('path_exists_after')!r}"
            )
    if "name_in_index_after" in expect:
        if bool(response.get("name_in_index_after")) != bool(expect["name_in_index_after"]):
            failures.append(
                f"name_in_index_after mismatch: expect={expect['name_in_index_after']!r} "
                f"got={response.get('name_in_index_after')!r}"
            )

    if "cross_language_paths_equal" in expect:
        # The cross-language equality is checked by the runner orchestrator; here we
        # only ensure the per-adapter response has a path.
        if not response.get("path"):
            failures.append("missing path on cross_language attach response")

    events = response.get("events", []) or []
    event_kinds = [e.get("kind") for e in events]
    for kind in expect.get("events_contain", []):
        if kind not in event_kinds:
            failures.append(f"events missing kind {kind!r}; got {event_kinds}")
    for kind in expect.get("events_exclude", []):
        if kind in event_kinds:
            failures.append(f"events should not contain kind {kind!r}; got {event_kinds}")

    return failures


# ---------------------------------------------------------------------------
# Cross-adapter differential check
# ---------------------------------------------------------------------------


def check_differential(responses: dict[str, dict[str, Any]], case: Case) -> list[str]:
    failures: list[str] = []
    skip_kinds: set[str] = set(case.expect.get("differential_ignore", []))
    responses = {k: v for k, v in responses.items() if not v.get("skipped")}
    if len(responses) < 2:
        return []

    if "ok" not in skip_kinds:
        oks = {n: bool(r.get("ok")) for n, r in responses.items()}
        if len(set(oks.values())) > 1:
            failures.append(f"ok differs: {oks}")

    if "error_kind" not in skip_kinds:
        any_error = False
        kinds: dict[str, str | None] = {}
        for n, r in responses.items():
            err = r.get("error")
            if err:
                any_error = True
            kinds[n] = err.get("kind") if err else None
        if any_error and len(set(kinds.values())) > 1:
            failures.append(f"error.kind differs: {kinds}")

    if "events" not in skip_kinds:
        multisets = {
            n: Counter(e.get("kind") for e in (r.get("events") or []))
            for n, r in responses.items()
        }
        first_n, first_ms = next(iter(multisets.items()))
        for n, ms in multisets.items():
            if ms != first_ms:
                failures.append(
                    f"event kind multiset differs between {first_n} and {n}: "
                    f"{first_n}={dict(first_ms)} {n}={dict(ms)}"
                )
                break

    return failures
