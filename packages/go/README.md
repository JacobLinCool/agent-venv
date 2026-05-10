# agent-venv (Go)

Go implementation of [`agent-venv`](https://github.com/JacobLinCool/agent-venv).
See the root README for the project overview and the [`spec/`](../../spec)
directory for the cross-language contract.

## Install

```bash
go get github.com/JacobLinCool/agent-venv/packages/go
```

Requires Go 1.25 or later.

## Layer 1 — generic environment

```go
package main

import (
    "context"
    "os"
    "os/exec"

    agentvenv "github.com/JacobLinCool/agent-venv/packages/go"
)

func main() {
    ctx := context.Background()
    spec := agentvenv.EnvironmentSpec{
        EnvOverrides: map[string]string{"CLAUDE_CONFIG_DIR": "$EPHEMERAL_HOME"},
    }
    env, err := agentvenv.NewEphemeral(ctx, spec)
    if err != nil {
        panic(err)
    }
    defer env.Destroy(ctx)

    cmd := exec.Command("claude", "--print", "hi")
    cmd.Env = os.Environ()
    for k, v := range env.EnvOverrides() {
        cmd.Env = append(cmd.Env, k+"="+v)
    }
    _ = cmd.Run()
}
```

The library never spawns the agent. The caller does. The library only
manages the profile dir and the env vars to point the agent at it.

## Layer 2 — built-in adapters

```go
env, err := agentvenv.NewEphemeralFor(ctx, agentvenv.ClaudeCode{})
defer env.Destroy(ctx)
```

The adapter copies your `~/.claude/.credentials.json` (or macOS Keychain
entry) into the new profile with mode `0600`. The original is not modified.

## Persistent registry

```go
env, _ := agentvenv.CreateOrAttach(ctx, "myapp-skill-x", spec)
names, _ := agentvenv.List(ctx)
_ = agentvenv.DestroyByName(ctx, "myapp-skill-x")
```

Persistent envs live under `$XDG_DATA_HOME/agent-venv/envs/` (or
`~/.local/share/agent-venv/envs/`). Override with `AGENT_VENV_REGISTRY_ROOT`
or `agentvenv.WithRegistryRoot(...)`.

## Errors

```go
_, err := agentvenv.Attach(ctx, "missing")
if errors.Is(err, agentvenv.ErrEnvironmentNotFound) {
    // ...
}
```

All errors are `*agentvenv.Error` with a `Kind` field matching the spec.
Compare via `errors.Is` against the package-level sentinels.

## Conformance

```bash
go build -o /tmp/avc ./cmd/agent-venv-conformance
/tmp/avc < requests.ndjson
```

See [`spec/conformance-protocol.md`](../../spec/conformance-protocol.md).
