import { promises as fs } from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import {
  AdapterMismatchError,
  ClaudeCode,
  Codex,
  Environment,
  EnvironmentNotFoundError,
  EnvironmentSpec,
  ProfileSetupFailedError,
} from "../../src/index.js";

let registryRoot = "";

beforeEach(async () => {
  registryRoot = await fs.mkdtemp(path.join(os.tmpdir(), "iso-ts-test-"));
});

afterEach(async () => {
  await fs.rm(registryRoot, { recursive: true, force: true });
});

describe("ephemeral", () => {
  it("creates, exposes path/env_overrides, destroys on dispose", async () => {
    let p: string;
    {
      await using env = await Environment.ephemeral({
        spec: new EnvironmentSpec({ envOverrides: { FOO: "$EPHEMERAL_HOME" } }),
      });
      p = env.path;
      expect(await fs.access(p).then(() => true).catch(() => false)).toBe(true);
      expect(env.envOverrides).toEqual({ FOO: p });
      expect(env.kind).toBe("ephemeral");
    }
    expect(await fs.access(p).then(() => true).catch(() => false)).toBe(false);
  });

  it("writes seed files", async () => {
    await using env = await Environment.ephemeral({
      spec: new EnvironmentSpec({ seedFiles: { "a.txt": "hi", "nested/b.txt": "yo" } }),
    });
    expect(await fs.readFile(path.join(env.path, "a.txt"), "utf-8")).toBe("hi");
    expect(await fs.readFile(path.join(env.path, "nested/b.txt"), "utf-8")).toBe("yo");
  });

  it("does NOT add HOME by default", async () => {
    await using env = await Environment.ephemeral({
      spec: new EnvironmentSpec({ envOverrides: { CLAUDE_CONFIG_DIR: "$EPHEMERAL_HOME" } }),
    });
    expect(env.envOverrides).toHaveProperty("CLAUDE_CONFIG_DIR");
    expect(env.envOverrides).not.toHaveProperty("HOME");
  });

  it("credentials default to mode 0600 on Unix", async () => {
    if (process.platform === "win32") return;
    await using env = await Environment.ephemeral({
      spec: new EnvironmentSpec({ credentials: { ".credentials.json": '{"k":"v"}' } }),
    });
    const stat = await fs.stat(path.join(env.path, ".credentials.json"));
    expect(stat.mode & 0o777).toBe(0o600);
  });

  it("rejects path traversal in seed files", async () => {
    await expect(
      Environment.ephemeral({
        spec: new EnvironmentSpec({ seedFiles: { "../escape": "bad" } }),
      }),
    ).rejects.toThrow(ProfileSetupFailedError);
  });
});

describe("persistent", () => {
  it("create_or_attach is idempotent", async () => {
    const spec = new EnvironmentSpec({ seedFiles: { "x.txt": "1" } });
    const e1 = await Environment.createOrAttach("E", { spec, registryRoot });
    const e2 = await Environment.createOrAttach("E", { spec, registryRoot });
    expect(e1.path).toBe(e2.path);
    expect(await fs.readFile(path.join(e2.path, "x.txt"), "utf-8")).toBe("1");
  });

  it("attach finds an existing env", async () => {
    const spec = new EnvironmentSpec();
    const e1 = await Environment.createOrAttach("E", { spec, registryRoot });
    const e2 = await Environment.attach("E", { registryRoot });
    expect(e2.path).toBe(e1.path);
  });

  it("attach missing throws EnvironmentNotFound", async () => {
    await expect(Environment.attach("nope", { registryRoot })).rejects.toThrow(EnvironmentNotFoundError);
  });

  it("list and destroy_by_name", async () => {
    const spec = new EnvironmentSpec();
    await Environment.createOrAttach("a", { spec, registryRoot });
    await Environment.createOrAttach("b", { spec, registryRoot });
    expect(await Environment.list({ registryRoot })).toEqual(["a", "b"]);
    await Environment.destroyByName("a", { registryRoot });
    expect(await Environment.list({ registryRoot })).toEqual(["b"]);
  });

  it("attach_mismatch raises", async () => {
    await Environment.createOrAttach("multi", {
      spec: new EnvironmentSpec({ adapterId: "claude-code" }),
      registryRoot,
    });
    await expect(
      Environment.createOrAttach("multi", {
        spec: new EnvironmentSpec({ adapterId: "codex" }),
        registryRoot,
      }),
    ).rejects.toThrow(AdapterMismatchError);
  });
});

describe("adapters", () => {
  it("ClaudeCode produces expected spec without credentials", async () => {
    const spec = await ClaudeCode({ loadCredentials: false }).buildSpec();
    expect(spec.adapterId).toBe("claude-code");
    expect(spec.envOverrides).toEqual({ CLAUDE_CONFIG_DIR: "$EPHEMERAL_HOME" });
    expect(spec.seedFiles[".claude.json"]).toBeDefined();
    expect(spec.credentials[".credentials.json"]).toBeUndefined();
  });

  it("Codex produces expected spec without credentials", async () => {
    const spec = await Codex({ loadCredentials: false }).buildSpec();
    expect(spec.envOverrides).toEqual({ CODEX_HOME: "$EPHEMERAL_HOME" });
  });

  it("Environment.ephemeral via adapter sets only the agent-specific var", async () => {
    await using env = await Environment.ephemeral({ adapter: ClaudeCode({ loadCredentials: false }) });
    expect(env.envOverrides).toHaveProperty("CLAUDE_CONFIG_DIR");
    expect(env.envOverrides).not.toHaveProperty("HOME");
  });
});
