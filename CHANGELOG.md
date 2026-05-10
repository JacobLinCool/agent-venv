# Changelog

All notable changes to `agent-venv` are documented here, sectioned by package.
Versions follow SemVer; see [`spec/compatibility.md`](spec/compatibility.md) for the policy.

The four packages MAY sit at different patch levels but SHOULD share the same
minor when the spec changes minor.

## [Unreleased]

### Python
-

### TypeScript
-

### Rust
-

### Go
- Initial Go implementation under `packages/go/`. Implements spec v0.1
  (Layer 1 generic environment + Layer 2 ClaudeCode/Codex adapters).
  Passes the cross-language conformance harness on `ubuntu-latest` and
  `macos-latest`. MSRV Go 1.25.

## [0.1.0] — 2026-05-10

Initial release of `agent-venv` across PyPI, npm, and crates.io. All three
implementations conform to the v0 spec in [`spec/`](spec/).

### Python
- `Environment.ephemeral`, `Environment.create_or_attach`, `Environment.attach`,
  `Environment.list`, `Environment.destroy_by_name`.
- `ClaudeCode` and `Codex` adapters.
- Conformance harness entry point at `python -m agent_venv.conformance`.

### TypeScript
- `Environment.ephemeral`, `Environment.createOrAttach`, `Environment.attach`,
  `Environment.list`, `Environment.destroyByName`. Supports `await using`.
- `ClaudeCode` and `Codex` adapters.
- `agent-venv-conformance` CLI bundled.

### Rust
- `Environment::ephemeral`, `Environment::create_or_attach`, `Environment::attach`,
  `Environment::list`, `Environment::destroy_by_name` — all async (Tokio).
- `ClaudeCode` and `Codex` adapters via the `AgentAdapter` trait.
- `agent-venv-conformance` binary bundled.
