# Contributing

Thanks for considering a contribution. `agent-venv` is a small polyglot library, but it's structured carefully because three language implementations are easy to drift apart.

## Before you start

Read [`spec/README.md`](spec/README.md) and [`agents/README.md`](agents/README.md). The spec is the source of truth, not any single implementation.

## What kind of change?

| Change | Path |
|---|---|
| Bug fix in one implementation | PR against `packages/<lang>/`, tests required, no spec change. |
| New feature, additive | Open an RFC under `rfcs/`. After acceptance, conformance cases first, implementations after. |
| Breaking spec change | RFC + minor version bump (pre-1.0). Discuss in an issue first. |
| New conformance case | PR under `conformance/cases/`. All three implementations must still pass. |
| Documentation fix | Direct PR. |

## RFC flow

1. Copy `rfcs/0000-template.md` to `rfcs/00NN-name.md`.
2. Fill it in.
3. Open a PR. Discussion happens there.
4. When two of the three Maintainers + Spec Steward agree, status flips to `accepted`.
5. Conformance Agent writes failing cases.
6. Maintainers implement in parallel.
7. All three pass → merge.

## Local setup

```bash
# Python
cd packages/python
uv pip install -e ".[dev]"
pytest tests/unit -v

# TypeScript
cd packages/typescript
pnpm install
pnpm test

# Rust
cd packages/rust
cargo test

# Go
cd packages/go
go test -race ./...

# Conformance (all four)
cd conformance/runner
uv pip install -e .
python -m agent_venv_conformance \
  --adapter python:python -m agent_venv.conformance \
  --adapter ts:node ../../packages/typescript/dist/conformance/bin.js \
  --adapter rust:../../packages/rust/target/debug/agent-venv-conformance \
  --adapter go:../../packages/go/agent-venv-conformance \
  --cases ../cases
```

## Style

Each implementation uses its language's standard formatter and linter. CI enforces:

- Python: `ruff format` + `ruff check` + `mypy`
- TypeScript: `prettier` + `eslint` + `tsc --noEmit`
- Rust: `cargo fmt` + `clippy -D warnings`
- Go: `gofmt` + `go vet` (`golangci-lint` recommended but not enforced)

## Commit messages

Conventional Commits style is preferred but not enforced.

## License

By contributing you agree your contribution is licensed under the project's MIT License.
