"""Reporting helpers for conformance results."""

from __future__ import annotations

import io
from typing import TYPE_CHECKING
from xml.sax.saxutils import escape, quoteattr

if TYPE_CHECKING:
    from .runner import Report


def render_text(report: "Report") -> str:
    out = io.StringIO()
    by_case: dict[str, list[str]] = {}
    pass_count = 0
    fail_count = 0
    skip_count = 0
    for o in report.per_adapter:
        if o.skipped:
            label = f"SKIP[{o.skip_reason or 'unspecified'}]"
            skip_count += 1
        elif o.passed:
            label = "PASS"
            pass_count += 1
        else:
            label = "FAIL"
            fail_count += 1
        by_case.setdefault(o.case_id, []).append(f"  {o.adapter}: {label}")
        if not o.passed and not o.skipped:
            for f in o.failures:
                by_case[o.case_id].append(f"    - {f}")

    diff_by_case: dict[str, list[str]] = {}
    diff_pass = 0
    diff_fail = 0
    for d in report.differential:
        if d.passed:
            diff_pass += 1
            diff_by_case.setdefault(d.case_id, []).append("  differential: PASS")
        else:
            diff_fail += 1
            diff_by_case.setdefault(d.case_id, []).append("  differential: FAIL")
            for f in d.failures:
                diff_by_case[d.case_id].append(f"    - {f}")

    out.write(f"adapters: {', '.join(report.adapters)}\n")
    out.write("\n")
    all_case_ids = sorted(set(by_case) | set(diff_by_case))
    for cid in all_case_ids:
        out.write(f"{cid}\n")
        for line in by_case.get(cid, []):
            out.write(line + "\n")
        for line in diff_by_case.get(cid, []):
            out.write(line + "\n")
    out.write("\n")
    out.write(
        f"per-adapter: pass={pass_count} fail={fail_count} skip={skip_count}\n"
    )
    if report.differential:
        out.write(f"differential: pass={diff_pass} fail={diff_fail}\n")
    out.write(f"OVERALL: {'PASS' if report.passed else 'FAIL'}\n")
    return out.getvalue()


def render_junit(report: "Report") -> str:
    out = io.StringIO()
    out.write('<?xml version="1.0" encoding="UTF-8"?>\n')

    by_adapter: dict[str, list] = {}
    for o in report.per_adapter:
        by_adapter.setdefault(o.adapter, []).append(o)

    out.write("<testsuites>\n")
    for adapter, outcomes in by_adapter.items():
        failures = sum(1 for o in outcomes if not o.passed and not o.skipped)
        skipped = sum(1 for o in outcomes if o.skipped)
        out.write(
            f'  <testsuite name={quoteattr(adapter)} tests="{len(outcomes)}" '
            f'failures="{failures}" skipped="{skipped}">\n'
        )
        for o in outcomes:
            out.write(
                f'    <testcase classname={quoteattr(adapter)} name={quoteattr(o.case_id)}>\n'
            )
            if o.skipped:
                out.write(f'      <skipped message={quoteattr(o.skip_reason or "")}/>\n')
            elif not o.passed:
                msg = "; ".join(o.failures)
                out.write(f'      <failure message={quoteattr(msg)}>{escape(msg)}</failure>\n')
            out.write("    </testcase>\n")
        out.write("  </testsuite>\n")

    if report.differential:
        diff_failures = sum(1 for d in report.differential if not d.passed)
        out.write(
            f'  <testsuite name="differential" tests="{len(report.differential)}" '
            f'failures="{diff_failures}">\n'
        )
        for d in report.differential:
            out.write(f'    <testcase classname="differential" name={quoteattr(d.case_id)}>\n')
            if not d.passed:
                msg = "; ".join(d.failures)
                out.write(f'      <failure message={quoteattr(msg)}>{escape(msg)}</failure>\n')
            out.write("    </testcase>\n")
        out.write("  </testsuite>\n")
    out.write("</testsuites>\n")
    return out.getvalue()
