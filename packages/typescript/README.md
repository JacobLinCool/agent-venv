# agent-venv (TypeScript)

TypeScript / Node implementation of [`agent-venv`](https://github.com/JacobLinCool/agent-venv). See the root README for the project overview and the [`spec/`](../../spec) directory for the cross-language contract.

## Install

```bash
npm install agent-venv
# or
pnpm add agent-venv
```

Requires Node 20+.

## Layer 1: any CLI

```ts
import { Sandbox } from "agent-venv";

await using sb = await Sandbox.create({
  policy: { maxRuntimeMs: 30_000 },
});
await sb.seed({ "main.js": "console.log('hi')" });
const out = await sb.run(["node", "main.js"]);
console.log(out.stdout);
```

The `await using` declaration calls `destroy()` automatically. If your runtime doesn't support it, call `await sb.destroy()` in a `finally`.

## Layer 2: built-in adapters

```ts
import { Sandbox, ClaudeCode } from "agent-venv";

await using sb = await Sandbox.withAgent(
  ClaudeCode({ model: "claude-haiku-4-5-20251001" }),
);
const out = await sb.runAgent({ prompt: "add a README" });
```

## Custom profile

```ts
import { Sandbox, ProfileSpec } from "agent-venv";

await using sb = await Sandbox.create({
  profile: ProfileSpec.ephemeralHome({
    envOverrides: { CLAUDE_CONFIG_DIR: "$EPHEMERAL_HOME" },
    seedFiles: { ".credentials.json": '{"oauth":"..."}' },
    fileModes: { ".credentials.json": 0o600 },
  }),
});
```

## Errors

```ts
import { Sandbox, AgentVenvError } from "agent-venv";

try {
  await using sb = await Sandbox.create({ policy: { maxRuntimeMs: 100 } });
  await sb.run(["sh", "-c", "sleep 5"]);
} catch (e) {
  if (e instanceof AgentVenvError && e.kind === "Timeout") {
    console.log("timed out");
  }
}
```

Errors carry a `kind` matching the spec.

## Conformance CLI

```bash
agent-venv-conformance < requests.ndjson
```

See [`spec/conformance-protocol.md`](../../spec/conformance-protocol.md).
