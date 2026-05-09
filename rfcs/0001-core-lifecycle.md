# RFC 0001: Environment-as-virtualenv core

- Status: accepted
- Author: initial spec
- Created: 2026-05-10
- Tracking issue: this is the v0 design

## Summary

`agent-venv` v0 is a polyglot library (Python, TypeScript, Rust) that provides **isolated profile directories** for coding-agent CLIs. Think `virtualenv` for `claude` and `codex`. The library does not run the agent; it gives the caller a directory and the env vars to point the agent at it.

## Motivation

Agent CLIs like `claude` and `codex` keep their settings, history, and credentials in a single per-user directory (`~/.claude/`, `~/.codex/`). When many apps and skills on a single host each invoke these CLIs for their own purpose, they all share — and step on — that one directory.

The need: each invocation (or each long-running skill profile) should get its own config directory, while still using the user's host **credentials** so it runs under the user's existing subscription.

## Detailed design

### What the library does

For each call to `Environment.ephemeral(spec)` or `Environment.create_or_attach(name, spec)`:

1. Create the profile directory (tempdir for ephemeral; `<registry_root>/envs/<slug>/profile/` for persistent).
2. Materialize seed files (markers, config files) per `spec.seed_files`.
3. Materialize credentials per `spec.credentials` (separate so `refresh_credentials()` can re-run only this part).
4. Apply `file_modes` (mode 0600 for credentials by default).
5. Resolve `$EPHEMERAL_HOME` placeholders in `spec.env_overrides` to the absolute profile path.
6. Return an `Environment` handle exposing `path`, `env_overrides`, `name`, `adapter_id`.

### What the library does NOT do

- Spawn the agent. The caller does, with the env vars `agent-venv` returns.
- Manage cwd / workspace. The caller decides where the agent runs.
- Implement timeouts, output limits, network policy, resource limits. None of these belong here.

### Two layers

**Layer 1 (Environment primitives)**: `Environment.ephemeral`, `Environment.create_or_attach`, `Environment.attach`, `Environment.list`, `Environment.destroy_by_name`. Generic — knows nothing about specific agent CLIs.

**Layer 2 (Adapters)**: `ClaudeCode`, `Codex`. Each is a small object that returns an `EnvironmentSpec` for its agent. Adapters are not privileged; users can write their own.

### Persistent registry

Persistent environments are tracked in `~/.local/share/agent-venv/envs/` (or `$XDG_DATA_HOME/...`). An `index.json` maps name → relative profile path. `metadata.json` per env records adapter_id, created_at, env_overrides. See `spec/registry.md`.

### Errors

`EnvironmentNotFound`, `AdapterMismatch`, `ProfileSetupFailed`, `RegistryUnavailable`, `CredentialsMissing`, `AdapterUnavailable`, `CleanupFailed`, `InvalidEnvironmentSpec`, `InternalInvariantViolation`. See `spec/errors.md`.

### Events

`env.created`, `env.attached`, `profile.materialized`, `credentials.copied`, `credentials.refreshed`, `env.destroyed`, `registry.read`, `registry.written`, `error`. See `spec/events.schema.json`.

### Conformance

Each language ships an adapter CLI that speaks NDJSON over stdin/stdout. The harness pipes the same case set into all three CLIs and asserts cross-language consistency. See `spec/conformance-protocol.md`.

## Drawbacks

- Three implementations means 3× the surface area and the risk of drift. The conformance suite mitigates but doesn't eliminate this.
- The persistent registry is a small-but-real piece of cross-process state. Concurrent writes need a lock; file-system corner cases (orphaned dirs, partial writes) need handling.

## Alternatives considered

- **Rust core + Python/Node bindings**: rejected. Native binding complexity (ABI, prebuild matrix, toolchain, debugging) is not justified by this workload, and would make each language's API feel like a wrapper rather than a native library.
- **Single Python implementation, defer TS/Rust**: deferred but not rejected. Cost of polyglot upfront accepted.

## Cross-language impact

- Python: `Environment` (sync) + `AsyncEnvironment` (async).
- TypeScript: async-only. `await using` for cleanup.
- Rust: async-only. Builder + `Drop` for ephemeral.

## Conformance

The full v0 case set lands in `conformance/cases/`:

- `lifecycle/` — ephemeral create / inspect / destroy.
- `persistent/` — create_or_attach idempotency, attach missing, list, destroy_by_name, attach mismatch, cross-language attach.
- `profile/` — file modes, env_overrides shape (agent-specific only, no HOME).

## Open questions

None for v0. Items explicitly deferred:

- HTTP API observation proxy (separate package).
- Container backend.
- A `create_strict` op that fails when the name exists.
- Credential rotation hooks (only manual `refresh_credentials()` is in v0).
