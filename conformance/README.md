# Conformance harness

Cross-language test harness. The Python runner pipes the same set of JSON cases into each language's adapter CLI and verifies they produce equivalent behavior.

## Run all cases against all three implementations

```bash
cd conformance/runner
uv pip install -e .
python -m agent_venv_conformance \
  --adapter python:python -m agent_venv.conformance \
  --adapter ts:node ../../packages/typescript/dist/conformance/bin.js \
  --adapter rust:../../packages/rust/target/debug/agent-venv-conformance \
  --cases ../cases
```

## Run against one adapter

```bash
python -m agent_venv_conformance \
  --adapter python:python -m agent_venv.conformance \
  --cases ../cases
```

## Output

Text summary on stdout. Optional JUnit XML via `--junit out.xml`.

## Layout

- `runner/` — the harness (Python).
- `cases/` — case files, organized by category.
- `programs/` — small portable scripts used as test fixtures.
- `golden/` — exact expected outputs (where applicable; v0 doesn't use these much).

See `spec/conformance-protocol.md` for the wire protocol.
