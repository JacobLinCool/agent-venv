# Error taxonomy

Every implementation MUST emit errors classified by these `kind` strings. The conformance suite asserts on `kind`. The actual exception class, error type, or enum variant is each language's choice.

| `kind` | Trigger |
|---|---|
| `EnvironmentNotFound` | `attach(name)` or `destroy_by_name(name)` for a name not in the registry. |
| `EnvironmentAlreadyExists` | Reserved. (Currently `create_or_attach` is idempotent so this isn't raised; reserved for a future `create` op that disallows attach.) |
| `AdapterMismatch` | `attach(name)` with an adapter whose `id` differs from what was recorded for that name. |
| `ProfileSetupFailed` | Could not materialize the profile dir, copy credentials, or write seed files. |
| `RegistryUnavailable` | Could not read or write the registry (lock failure, permission, corrupted index). |
| `CredentialsMissing` | The adapter requires host credentials but they aren't present (Keychain entry missing, `~/.codex/auth.json` missing, …). Distinct from `ProfileSetupFailed` so callers can prompt the user to log in. |
| `AdapterUnavailable` | Adapter's CLI binary is not on PATH. Only raised when the caller explicitly calls `adapter.ensure_available()`; `Environment.create()` does NOT call this implicitly because the library doesn't run the binary. |
| `CleanupFailed` | `destroy()` could not remove the profile dir, but the registry was updated (for persistent envs). |
| `InvalidEnvironmentSpec` | The spec passed to `create*` is malformed (e.g., absolute path in seed files, conflicting env_overrides). |
| `InternalInvariantViolation` | A bug, or misuse (using a destroyed env handle). |

## Per-language idiom

- **Python**: `AgentVenvError(Exception)` is the base. Subclasses are named `EnvironmentNotFoundError`, etc., with the `Error` suffix.
- **TypeScript**: `AgentVenvError extends Error` is the base. The `kind` field is the discriminator.
- **Rust**: `enum Error { EnvironmentNotFound { .. }, ... }` with `thiserror::Error`. Each variant has a `kind() -> &'static str`.

## Error payloads

Errors carry structured context. The conformance suite checks the `kind` only; payloads are language-friendly but should be informative.

| `kind` | Suggested payload fields |
|---|---|
| `EnvironmentNotFound` | `name`, `registry_root` |
| `AdapterMismatch` | `name`, `expected_adapter_id`, `actual_adapter_id` |
| `ProfileSetupFailed` | `reason`, `path` (optional) |
| `RegistryUnavailable` | `reason`, `path` |
| `CredentialsMissing` | `adapter_id`, `looked_at` (list of paths/keychain entries tried) |
| `AdapterUnavailable` | `adapter_id`, `cli_bin` |
| `CleanupFailed` | `path`, `os_error` |
| `InvalidEnvironmentSpec` | `field`, `reason` |
| `InternalInvariantViolation` | `message` |
