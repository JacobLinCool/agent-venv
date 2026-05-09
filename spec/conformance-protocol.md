# Conformance protocol

The conformance harness verifies every implementation behaves identically. It runs each language's adapter CLI as a subprocess and exchanges JSON over stdin/stdout.

## Adapter CLI contract

Each implementation ships a small CLI:

| Implementation | Invocation |
|---|---|
| Python | `python -m agent_venv.conformance` |
| TypeScript | `agent-venv-conformance` (npm bin) or `node dist/conformance/bin.cjs` |
| Rust | `agent-venv-conformance` (cargo bin) |

The CLI:

1. Prints exactly one line `{"protocol":"agent-venv.conformance","version":2,"language":"<lang>",...}` to stdout, then flushes.
2. Reads one JSON object per line from stdin (newline-delimited JSON).
3. For each request line, executes the request and writes one JSON response line, then flushes.
4. Exits 0 on EOF on stdin.

The protocol version is **2** for the environment-management contract (version 1 was the process-management contract from an earlier draft).

## Request shape

```json
{
  "case_id": "001-ephemeral-lifecycle",
  "op": "ephemeral_lifecycle",
  "spec": {
    "adapter_id": "generic",
    "env_overrides": {"FOO": "$EPHEMERAL_HOME"},
    "seed_files": {"a.txt": "1"},
    "file_modes": {"a.txt": 384},
    "credentials": {}
  }
}
```

Or for persistent ops:

```json
{
  "case_id": "010-persistent-attach-idempotent",
  "op": "persistent_create_attach_idempotent",
  "name": "test-env-1",
  "registry_root": "/tmp/iso-conformance-XYZ",
  "spec": { "adapter_id": "generic", "env_overrides": {"FOO": "$EPHEMERAL_HOME"} }
}
```

The harness creates a fresh `registry_root` per case and includes it in the request. It cleans up after.

## Operations (v0)

| `op` | Behaviour | Response shape (in addition to standard fields) |
|---|---|---|
| `ephemeral_lifecycle` | create ephemeral env, inspect it, destroy. | `inspection`: `{path, env_overrides, files_present, file_modes, exists}`; `after_destroy`: `{path_exists}` |
| `persistent_create_attach_idempotent` | call `create_or_attach(name)` twice with same args. | `paths`: `[path1, path2]` |
| `persistent_attach_missing` | `attach(name)` on a name that doesn't exist. | error.kind = `EnvironmentNotFound` |
| `persistent_destroy_by_name` | create then `destroy_by_name(name)`; verify gone. | `created_path`, `path_exists_after`: false, `name_in_index_after`: false |
| `persistent_list` | create two persistent envs in the same root, list. | `names`: sorted array |
| `persistent_attach_mismatch` | create with adapter A then `attach(name)` with adapter B. | error.kind = `AdapterMismatch` |
| `cross_language_attach` | another language created the env at this root; this adapter must `attach` and yield the same `path`. (Used as a multi-step orchestration in the harness; per-adapter response is just `{path}`.) | `path` |

## Response shape

```json
{
  "case_id": "001-ephemeral-lifecycle",
  "ok": true,
  "events": [
    {"ts_ms": 0, "kind": "env.created"},
    {"ts_ms": 1, "kind": "profile.materialized"},
    {"ts_ms": 5, "kind": "env.destroyed"}
  ],
  "inspection": {
    "path": "/tmp/...",
    "env_overrides": {"FOO": "/tmp/..."},
    "files_present": ["a.txt"],
    "file_modes": {"a.txt": 384},
    "exists": true
  },
  "after_destroy": {"path_exists": false}
}
```

`ok` is `true` if the case ran (whether or not it produced an error of the *expected* kind). `ok` is `false` only if the adapter failed to interpret or set up the request itself.

If the operation raises an `AgentVenvError`, the response carries:
```json
{ "ok": true, "error": {"kind": "EnvironmentNotFound", "message": "..."} }
```

## Differential checking

Across implementations, the harness asserts equality on:

- `ok`
- `error.kind` if any adapter reported an error
- `events[*].kind` as a multiset
- For `persistent_create_attach_idempotent`: `paths[0] == paths[1]` within each adapter (path equality is intra-adapter; cross-adapter paths can differ).

Per-case `expect` blocks add specific assertions (file presence, exact env var keys, etc.).

## Cross-language op

`cross_language_attach` is special: it runs in two stages. Stage 1 picks a "creator" adapter (rotated per case) to create a persistent env. Stage 2 sends `attach` to every other adapter pointing at the same `registry_root` and `name`. All MUST return the same `path` and the same `env_overrides` (modulo absolute-path differences that are unavoidable but not present here because the path IS the result).
