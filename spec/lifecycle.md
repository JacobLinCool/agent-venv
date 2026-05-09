# Environment lifecycle

An environment is a finite state machine. There are two flavors that share most states.

## Ephemeral

```
Created → Active → Destroyed
```

| State | What it means |
|---|---|
| `Created` | The profile directory exists. Credentials (if requested) and seed files are materialized inside it. `env_overrides` are computed. |
| `Active` | The same as `Created`; the env is usable. The library does not track agent invocations against the env — there is no "Running" state. The caller may spawn the agent zero or more times against this env. |
| `Destroyed` | The profile directory is removed. The env handle is no longer usable. |

Ephemeral environments are typically used inside a context manager (`with` / `await using` / `Drop`) and disappear when the block exits.

## Persistent

```
Created ⇄ Detached → Reattached → … → Destroyed
```

| State | What it means |
|---|---|
| `Created` | First time `create_or_attach(name)` is called. Profile dir, credentials, seed files, registry entry are materialized. |
| `Detached` | The handle has been dropped (process exited, etc.) but the on-disk state remains. The env is referenceable by name. |
| `Reattached` | A subsequent `create_or_attach(name)` or `attach(name)` returns a handle backed by the same on-disk path. Credentials are *not* re-copied unless `refresh_credentials()` is called. |
| `Destroyed` | The profile dir AND the registry entry are removed. The name is now free. |

`create_or_attach` is idempotent: same `name`, same `registry_root`, same `adapter` → same path, no duplicate entries.

`attach(name)` raises `EnvironmentNotFound` if the name is not in the registry.

`list()` enumerates persistent environments by reading the registry.

`destroy_by_name(name)` removes the env from disk and registry without needing a live handle.

## Operations contract

- `create(spec)` for ephemeral: makes a fresh tempdir-based env.
- `create_or_attach(name, spec, registry_root?)` for persistent: idempotent.
- `attach(name, registry_root?)` for persistent: never creates; raises if missing.
- `list(registry_root?)` for persistent: enumerate.
- `destroy()` on a handle: ephemeral always removes; persistent also removes registry entry.
- `destroy_by_name(name, registry_root?)`: same as above without a handle.
- `refresh_credentials()`: re-copy credentials from host into the env.

## Idempotency rules

- `destroy()` is idempotent. Calling it twice is a no-op the second time.
- `create_or_attach` with same args is idempotent in identity (same path) but does not re-materialize seed files. If the profile dir was tampered with externally, the library does not "heal" it.
- The destructor of the language's resource type calls `destroy()` only for ephemeral envs. Persistent envs survive their handles by design.

## Adapter selection on attach

When attaching to a persistent env, the registry records the `adapter_id` used at creation. If the caller's `attach()` provides an adapter that disagrees, the implementation MUST raise `AdapterMismatch`. This prevents code that expects a Claude Code env from accidentally inheriting a Codex env with the same name.

## Refreshing credentials

`refresh_credentials()` re-runs the adapter's credential loading and overwrites the credential files in the profile dir. This is the supported way to recover after the user's host credentials rotate. It does not touch other files in the profile.
