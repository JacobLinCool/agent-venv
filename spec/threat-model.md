# Threat model

`agent-venv` is a **profile isolation layer** for coding-agent CLIs. It is not a sandbox.

## What we protect against

The library's job is to make it safe for many independent apps and skills to invoke `claude`, `codex`, etc. from the same machine without:

1. **Cross-app config pollution.** App A using `claude` for one skill must not have its `~/.claude/` state changed by App B running a different skill. Each gets its own `CLAUDE_CONFIG_DIR`.
2. **Damage to the user's primary profile.** A misbehaving agent invocation must not be able to corrupt the user's real `~/.claude/` or `~/.codex/`. We achieve this by setting `CLAUDE_CONFIG_DIR` / `CODEX_HOME` (and friends) to a directory we own.
3. **Credential bleed.** When the user opts in to `load_credentials=True`, credentials are copied (not symlinked) into the new profile dir, with file mode `0600` on Unix. Modifications inside the profile never propagate back to `~/.claude/`.
4. **Stale state across long-lived environments.** Persistent environments are keyed by name and stored in a registry. Re-attaching to the same name returns the same path; conflicting concurrent creators see consistent behavior.
5. **Cleanup correctness.** Ephemeral environments are removed on `destroy()`. Destroying a persistent environment removes it from the registry and from disk.

## What we explicitly do NOT protect against

- **Anything the agent does once it's running.** We do not run the agent. The caller does. The caller picks `cwd`, `argv`, network access, timeouts, output handling. None of that is our scope.
- **Filesystem escape.** A determined process inside the profile dir can `cd ..`, follow symlinks out, read host files, etc. We override env vars to point well-behaved tools at the right place; we do not prevent malicious behavior.
- **Network policy.** The agent has the host's full network access. There is no `allow_network` switch.
- **Memory / CPU caps.** The caller spawns the process; the caller decides resource limits if any.
- **Privilege escalation.** No chroot, no namespaces, no seccomp.

## Adversary model

We assume the agent is **not adversarial** but **not trusted to be careful with config**. It might write garbage into its config dir; we want that garbage to land somewhere disposable. The caller (the app embedding `agent-venv`) is fully trusted.

## Why this scope

The library was extracted from a benchmark that needed to run thousands of independent agent invocations against the user's one set of credentials, without trampling each other. As use cases broadened — Skill-as-a-service apps, agent-as-a-service deployments — the same need showed up: many invocations from the same host, sharing one user's credentials, but otherwise completely separate.

This is a thin, focused responsibility: profile dir + env vars + lifecycle. Larger goals (resource caps, syscall sandboxing, container backends) are different layers; users compose `agent-venv` with them when they need them.
