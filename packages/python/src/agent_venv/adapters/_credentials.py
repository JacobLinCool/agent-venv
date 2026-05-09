"""Credential helpers for built-in adapters.

These read OAuth credentials from the user's host so they can be
materialized inside an ephemeral HOME for the duration of one sandbox run.
The originals are not modified.
"""

from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path

from ..errors import ProfileSetupFailedError


def read_claude_credentials() -> str:
    """Return Claude Code OAuth credentials JSON as text.

    On macOS the canonical store is the system Keychain entry
    ``Claude Code-credentials``; on other platforms it's
    ``~/.claude/.credentials.json``.
    """

    if sys.platform == "darwin":
        try:
            res = subprocess.run(
                [
                    "security",
                    "find-generic-password",
                    "-s",
                    "Claude Code-credentials",
                    "-a",
                    os.environ.get("USER", ""),
                    "-w",
                ],
                capture_output=True,
                text=True,
                check=True,
                timeout=10,
            )
        except (subprocess.CalledProcessError, subprocess.TimeoutExpired):
            res = None
        if res is not None and res.stdout.strip():
            return res.stdout.rstrip("\n") + "\n"

    src = Path.home() / ".claude" / ".credentials.json"
    if src.exists():
        return src.read_text(encoding="utf-8")
    raise ProfileSetupFailedError(
        "Claude Code credentials not found in macOS Keychain "
        "('Claude Code-credentials') or ~/.claude/.credentials.json. "
        "Run `claude` once to log in before running.",
        reason="claude_credentials_missing",
    )


def read_codex_auth() -> str:
    src = Path.home() / ".codex" / "auth.json"
    if not src.exists():
        raise ProfileSetupFailedError(
            "~/.codex/auth.json not found. Run `codex` once to log in before running.",
            reason="codex_auth_missing",
            path=str(src),
        )
    return src.read_text(encoding="utf-8")
