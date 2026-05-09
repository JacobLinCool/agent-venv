# agent-venv (Python)

Python implementation of [`agent-venv`](https://github.com/JacobLinCool/agent-venv). See the root README for the project overview and the [`spec/`](../../spec) directory for the cross-language contract.

## Install

```bash
pip install agent-venv
# or, from this monorepo:
uv pip install -e .
```

Requires Python 3.10+.

## Layer 1 — generic environment

```python
import subprocess
from agent_venv import Environment, EnvironmentSpec

# Ephemeral: cleaned up on context exit
with Environment.ephemeral(
    EnvironmentSpec(env_overrides={"CLAUDE_CONFIG_DIR": "$EPHEMERAL_HOME"})
) as env:
    subprocess.run(
        ["claude", "--print", "hi"],
        env={**os.environ, **env.env_overrides},
        cwd="/anywhere",
    )

# Persistent: keyed by name, survives the process
env = Environment.create_or_attach(
    "myapp-skill-x",
    EnvironmentSpec(env_overrides={"CLAUDE_CONFIG_DIR": "$EPHEMERAL_HOME"}),
)
print(env.path, env.env_overrides)
# ...later, from anywhere...
env = Environment.attach("myapp-skill-x")
```

The library **never spawns the agent**. The caller does. The library only manages the profile dir and the env vars to point the agent at it.

## Layer 2 — built-in adapters

`ClaudeCode` and `Codex` are thin wrappers that produce the right `EnvironmentSpec`, including reading host credentials so the agent runs under your subscription.

```python
from agent_venv import Environment, ClaudeCode

with Environment.ephemeral(adapter=ClaudeCode()) as env:
    subprocess.run(
        ["claude", "--print", "hi"],
        env={**os.environ, **env.env_overrides},
    )
```

The adapter copies your `~/.claude/.credentials.json` (or macOS Keychain entry) into the new profile with mode `0600`. The original is not modified.

## Persistent registry

Persistent envs live under `$XDG_DATA_HOME/agent-venv/envs/` (or `~/.local/share/agent-venv/envs/`). Override with `AGENT_VENV_REGISTRY_ROOT` env var or per-call `registry_root=`.

```python
Environment.list()                          # all persistent env names
Environment.attach("name")                  # raises if missing
Environment.destroy_by_name("name")         # removes from disk + registry
env.refresh_credentials()                   # re-copy host credentials
```

## Async

```python
from agent_venv import AsyncEnvironment

async with await AsyncEnvironment.ephemeral(adapter=ClaudeCode()) as env:
    ...
```

## Errors

```python
from agent_venv import errors

try:
    Environment.attach("missing")
except errors.EnvironmentNotFoundError as exc:
    print(exc.kind, exc)  # "EnvironmentNotFound", "..."
```

All errors subclass `errors.AgentVenvError` and carry a `kind: str` matching the spec.

## Conformance

```bash
python -m agent_venv.conformance < requests.ndjson
```

See [`spec/conformance-protocol.md`](../../spec/conformance-protocol.md).
