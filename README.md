# agent-venv

`virtualenv` for coding-agent CLIs. When apps and skills on your machine each invoke `claude` or `codex`, they shouldn't have to share — or trample on — your single `~/.claude/` and `~/.codex/`. `agent-venv` gives each invocation its own profile directory, copies your host credentials into it (so it still uses your subscription), and hands the caller back the env vars to point the agent at it.

The library **does not run the agent**. It manages the environment. The caller spawns `claude` / `codex` themselves with the env vars `agent-venv` returns.

Three idiomatic implementations from one shared spec:

- **Python** — `pip install agent-venv` (`packages/python/`)
- **TypeScript** — `npm install agent-venv` (`packages/typescript/`)
- **Rust** — `cargo add agent-venv` (`packages/rust/`)

Each is a first-class library. There is no native core; no binding layer. The [`spec/`](spec/) is authoritative.

## Two flavors

**Ephemeral** — created in a `with` / `await using` block, destroyed when the block exits. For short-lived, one-shot invocations.

**Persistent** — keyed by name, stored under `~/.local/share/agent-venv/envs/`, survives across processes. `create_or_attach(name, ...)` is idempotent; calling it twice with the same name returns the same path. For long-lived skill profiles that should keep their settings/history between runs.

## Two layers

**Layer 1** — `Environment.ephemeral`, `Environment.create_or_attach`, `Environment.attach`, `Environment.list`, `Environment.destroy_by_name`. Generic. Takes an `EnvironmentSpec` (env_overrides + seed_files + credentials + file_modes). Knows nothing about specific agent CLIs.

**Layer 2** — `ClaudeCode`, `Codex` adapters. Thin convenience that returns the right `EnvironmentSpec` for an agent. Adapters are not privileged; users can write their own.

## Quickstart

### Python

```python
import os, subprocess
from agent_venv import Environment, ClaudeCode

# Ephemeral
with Environment.ephemeral(adapter=ClaudeCode()) as env:
    subprocess.run(
        ["claude", "--print", "hi"],
        env={**os.environ, **env.env_overrides},
        cwd="/wherever/you/want",
    )

# Persistent, named, reattachable
env = Environment.create_or_attach("myapp-skill-x", adapter=ClaudeCode())
print(env.path, env.env_overrides)
# ...later, from another process...
env = Environment.attach("myapp-skill-x")
```

### TypeScript

```ts
import { spawn } from "node:child_process";
import { Environment, ClaudeCode } from "agent-venv";

await using env = await Environment.ephemeral({ adapter: ClaudeCode() });
spawn("claude", ["--print", "hi"], {
  env: { ...process.env, ...env.envOverrides },
});

const persistent = await Environment.createOrAttach("myapp-skill-x", {
  adapter: ClaudeCode(),
});
```

### Rust

```rust
use agent_venv::{Environment, adapters::{AgentAdapter, ClaudeCode}};

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let spec = ClaudeCode::new().build_spec()?;
    let mut env = Environment::ephemeral(spec).await?;
    println!("CLAUDE_CONFIG_DIR={}", env.env_overrides()["CLAUDE_CONFIG_DIR"]);
    // ... caller spawns `claude` with env.env_overrides() merged into env ...
    env.destroy().await?;
    Ok(())
}
```

The library only sets the **agent-specific** env var (`CLAUDE_CONFIG_DIR`, `CODEX_HOME`); it does not touch `HOME` so other tools that share your shell environment are unaffected.

## Status

v0. The spec is stable enough to implement against, but minor versions may still adjust additively. See [`spec/compatibility.md`](spec/compatibility.md).

## Repository layout

```
spec/                  # the authoritative spec — read this first
rfcs/                  # design proposals, accepted and pending
conformance/           # the cross-language test harness + JSON cases
packages/python/       # Python implementation
packages/typescript/   # TypeScript implementation
packages/rust/         # Rust implementation
agents/                # maintainer role briefs (humans + AI)
benchmarks/            # cross-language perf comparisons (v0 stub)
security/              # red-team findings (v0 stub)
```

## Development

See [`CONTRIBUTING.md`](CONTRIBUTING.md).

## License

MIT
