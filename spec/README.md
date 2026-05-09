# agent-venv specification

This directory is the **single source of truth** for what `agent-venv` does. The Python, TypeScript, and Rust implementations under `packages/` are equal first-class citizens — none of them is the reference implementation. The spec is.

If a behavior is not described here, it is not part of `agent-venv`. If two implementations disagree, the spec wins; the implementation is wrong.

## What this library is

`agent-venv` manages **isolated profiles** for coding-agent CLIs (Claude Code, Codex, …). Think of it as `virtualenv` for agents.

When an app on your machine wants to invoke `claude` or `codex` for a particular skill or role, it asks `agent-venv` for an environment. The library:

1. creates a directory that will serve as that agent's `CLAUDE_CONFIG_DIR` / `CODEX_HOME`,
2. (optionally) copies the user's host credentials into it so the agent runs under the user's subscription,
3. returns an `Environment` handle exposing `path` and `env_overrides`,
4. lets the caller spawn `claude` / `codex` themselves with those env vars,
5. cleans up when the env is destroyed (ephemeral) or persists it for reattachment (persistent).

The library **never spawns the agent itself**. Where the caller runs the agent (cwd, args, prompt) is the caller's business.

## Reading order

1. [`threat-model.md`](threat-model.md) — what we are and are not protecting against.
2. [`lifecycle.md`](lifecycle.md) — the state machine of an environment, ephemeral and persistent variants.
3. [`environment-spec.schema.json`](environment-spec.schema.json) — JSON Schema for `EnvironmentSpec`.
4. [`registry.md`](registry.md) — how persistent environments are stored and looked up.
5. [`errors.md`](errors.md) — error taxonomy.
6. [`events.schema.json`](events.schema.json) — audit event schema.
7. [`adapter-protocol.md`](adapter-protocol.md) — what an adapter contributes.
8. [`conformance-protocol.md`](conformance-protocol.md) — JSON-line stdin/stdout protocol for cross-language testing.
9. [`compatibility.md`](compatibility.md) — semver and breaking-change policy.

## Two-layer API contract

Every implementation MUST expose two layers:

**Layer 1 — Generic environment primitives.** `Environment.ephemeral(spec)`, `Environment.create_or_attach(name, spec)`, `Environment.attach(name)`, `Environment.list()`, `Environment.destroy_by_name(name)`. Knows nothing about specific agent CLIs.

**Layer 2 — Built-in adapters.** `ClaudeCode`, `Codex`. Each is a function that returns an `EnvironmentSpec` (and, optionally, an `argv_helper()` for callers who want help building the agent's command line). Adapters are not privileged; users can write their own.
