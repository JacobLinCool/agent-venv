# Adapter protocol (Layer 2)

An adapter is a small object that knows three things about a particular agent CLI:

1. **Where its credentials live on the host** and what they look like.
2. **Which env var the CLI reads to find its config dir** (so we can override it).
3. **(Optional) How to assemble argv** for callers who want help — but the library itself never invokes the binary.

That's it. An adapter is a pure mapping from "I want a profile for Claude Code" to an `EnvironmentSpec`.

## Required interface

Each implementation defines `AgentAdapter` (Python ABC, TypeScript interface, Rust trait):

```text
trait AgentAdapter:
    id: str                 // stable identifier, e.g. "claude-code"
    cli_bin: str            // binary name on PATH, e.g. "claude" — informational
    config_env_var: str     // e.g. "CLAUDE_CONFIG_DIR"

    // Build the EnvironmentSpec the library should materialize.
    // The adapter decides:
    //   - which env var(s) to put in env_overrides (typically just config_env_var)
    //   - what credentials to load from the host (or empty if load_credentials=False)
    //   - what onboarding/marker files to seed
    fn build_spec(load_credentials: bool) -> EnvironmentSpec

    // Optional: build argv for callers who want help. Library never calls this.
    fn build_argv(prompt: str, workspace: Path?) -> list[str]

    // Optional: verify the CLI is on PATH. Caller's choice; not invoked by create*().
    fn ensure_available() -> Result<()>
```

`build_spec` MUST set `env_overrides` to point the agent at `$EPHEMERAL_HOME`, e.g. `{"CLAUDE_CONFIG_DIR": "$EPHEMERAL_HOME"}`. It SHOULD NOT add `HOME`. The library will resolve `$EPHEMERAL_HOME` to the absolute profile path at creation time.

## Built-in adapters

### `ClaudeCode`

| Aspect | Value |
|---|---|
| `id` | `"claude-code"` |
| `cli_bin` | `"claude"` |
| `config_env_var` | `"CLAUDE_CONFIG_DIR"` |
| Credential source (load=true) | `Claude Code-credentials` macOS Keychain entry, falling back to `~/.claude/.credentials.json` |
| Credentials placed at | `.credentials.json` (mode 0600) |
| Onboarding marker (load=false) | `.claude.json` containing `{"hasCompletedOnboarding": true}` |
| `env_overrides` | `{"CLAUDE_CONFIG_DIR": "$EPHEMERAL_HOME"}` |

### `Codex`

| Aspect | Value |
|---|---|
| `id` | `"codex"` |
| `cli_bin` | `"codex"` |
| `config_env_var` | `"CODEX_HOME"` |
| Credential source (load=true) | `~/.codex/auth.json` (no Keychain) |
| Credentials placed at | `auth.json` (mode 0600) |
| Onboarding marker | none |
| `env_overrides` | `{"CODEX_HOME": "$EPHEMERAL_HOME"}` |

## What Layer 2 deliberately does NOT include

The bnf code from which `agent-venv` was extracted included additional adapter responsibilities: token/byte counting via stdout parsing, an HTTP proxy for vendor API calls, `wire_proxy()` for endpoint redirection, and process management. None of that is part of `agent-venv` — those are observability and execution concerns that belong in separate packages.

## Writing your own adapter

Users add adapters by implementing the protocol above in their own code; nothing about Layer 2 is privileged. A well-formed user adapter is indistinguishable from `ClaudeCode` and `Codex` from Layer 1's perspective.
