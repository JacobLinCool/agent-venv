# Persistent environment registry

Persistent environments are tracked in a small filesystem-based registry so they can be looked up by name across processes and across language implementations.

## Default location

```
$XDG_DATA_HOME/agent-venv/envs/        (Unix; falls back to ~/.local/share/...)
~/Library/Application Support/agent-venv/envs/  (macOS, optional override)
```

Implementations MUST resolve the default like:

1. If `AGENT_VENV_REGISTRY_ROOT` is set, use that.
2. Else if `XDG_DATA_HOME` is set, use `$XDG_DATA_HOME/agent-venv/envs/`.
3. Else use `~/.local/share/agent-venv/envs/`.

The default is overridable per call by passing `registry_root`.

## Layout

```
<registry_root>/
├── index.json            # name → relative_dir
└── envs/
    ├── <slug-1>/
    │   ├── metadata.json
    │   └── profile/      # this is what CLAUDE_CONFIG_DIR / CODEX_HOME points at
    └── <slug-2>/
        ├── metadata.json
        └── profile/
```

`<slug>` is a filesystem-safe hash of the name (e.g. SHA-256 truncated to 16 hex chars). Implementations MAY use a different slug scheme as long as it's stable and conflict-free; the conformance harness does not check the exact slug, only that resolving the same name yields the same absolute profile path.

## `metadata.json`

```json
{
  "schema_version": 1,
  "name": "myapp-front-end",
  "adapter_id": "claude-code",
  "created_at": "2026-05-10T12:00:00Z",
  "env_overrides": {"CLAUDE_CONFIG_DIR": "<absolute path>"},
  "credentials_loaded": true,
  "credentials_loaded_at": "2026-05-10T12:00:00Z"
}
```

`env_overrides` is recorded with absolute paths (the library resolved any `$EPHEMERAL_HOME` placeholders at creation time).

## `index.json`

```json
{
  "schema_version": 1,
  "entries": {
    "myapp-front-end": "envs/2f5a3b1c8d/",
    "research-skill": "envs/9b1cd0e3f7/"
  }
}
```

The directory paths are relative to the registry root.

## Concurrency

In v0, implementations SHOULD use a simple lock file at `<registry_root>/.lock` (advisory `flock` on Unix) to serialize writes to `index.json`. Reads are best-effort: a stale read of `index.json` is acceptable as long as the on-disk profile dir is still resolvable from the slug.

A `create_or_attach` race where two processes try to create the same name MUST resolve to one canonical entry — both callers see the same path, and at most one entry exists in the index. The simplest correct implementation: under the lock, re-read `index.json`, then either insert-and-create or use the existing entry.

## Cross-language sharing

Because the registry is plain JSON files on a known path, a Python process can create a persistent env that a Rust process later attaches to. The conformance suite verifies this: it creates an env with one language and attaches with another.

## Cleanup

`destroy_by_name(name)`:
1. Acquire lock.
2. Read `index.json`. If `name` not present, raise `EnvironmentNotFound`.
3. Remove the on-disk profile dir.
4. Remove the entry from `index.json`. Write atomically (write to tmp + rename).
5. Release lock.

If step 3 fails partially, step 4 still runs (the entry is dropped from the index). The caller gets `CleanupFailed` with the underlying error, but the registry is consistent.
