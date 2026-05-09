from __future__ import annotations

import argparse
import shlex
import sys
from pathlib import Path

from .runner import AdapterCLI, Runner, load_cases
from .report import render_text, render_junit


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="agent-venv-conformance")
    parser.add_argument(
        "--adapter",
        action="append",
        required=True,
        metavar="NAME:CMD",
        help="Adapter CLI to test, e.g. 'python:python -m agent_venv.conformance'. "
        "Repeatable. The CMD is shell-split.",
    )
    parser.add_argument(
        "--cases",
        type=Path,
        required=True,
        help="Directory containing case JSON files (recursive).",
    )
    parser.add_argument("--junit", type=Path, default=None, help="Write JUnit XML here.")
    parser.add_argument(
        "--filter",
        default=None,
        help="Substring filter on case_id; only matching cases run.",
    )
    parser.add_argument(
        "--no-differential",
        action="store_true",
        help="Skip cross-adapter differential checks.",
    )
    parser.add_argument(
        "--verbose", "-v", action="count", default=0, help="Increase verbosity."
    )
    args = parser.parse_args(argv)

    adapters: list[AdapterCLI] = []
    for spec in args.adapter:
        if ":" not in spec:
            parser.error(f"--adapter must be NAME:CMD, got {spec!r}")
        name, _, cmd = spec.partition(":")
        adapters.append(AdapterCLI(name=name, command=shlex.split(cmd)))

    cases = load_cases(args.cases)
    if args.filter:
        cases = [c for c in cases if args.filter in c.case_id]
    if not cases:
        print("no cases matched", file=sys.stderr)
        return 2

    runner = Runner(adapters=adapters, cases=cases, differential=not args.no_differential)
    report = runner.run(verbose=args.verbose)
    print(render_text(report))
    if args.junit:
        args.junit.write_text(render_junit(report), encoding="utf-8")
    return 0 if report.passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
