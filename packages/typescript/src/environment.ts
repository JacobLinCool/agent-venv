import { promises as fs } from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import {
  CleanupFailedError,
  EnvironmentNotFoundError,
  InternalInvariantViolationError,
  InvalidEnvironmentSpecError,
  ProfileSetupFailedError,
} from "./errors.js";
import { Event, EventLog, EventSink } from "./events.js";
import { materializeProfile, removeDir } from "./profile.js";
import { Registry, defaultRegistryRoot } from "./registry.js";
import { EnvironmentSpec } from "./spec.js";
import type { AgentAdapter } from "./adapters/base.js";

export interface EnvironmentOptions {
  spec?: EnvironmentSpec;
  adapter?: AgentAdapter;
  onEvent?: EventSink;
}

export interface PersistentOptions extends EnvironmentOptions {
  registryRoot?: string;
}

interface BackingState {
  path: string;
  envOverrides: Record<string, string>;
  spec: EnvironmentSpec;
  log: EventLog;
  kind: "ephemeral" | "persistent";
  name: string | null;
  registry: Registry | null;
  destroyed: boolean;
}

export class Environment {
  private state: BackingState;

  private constructor(state: BackingState) {
    this.state = state;
  }

  // ------------------------------------------------------------------
  // Construction
  // ------------------------------------------------------------------

  static async ephemeral(opts: EnvironmentOptions = {}): Promise<Environment> {
    const spec = await resolveSpec(opts);
    const log = new EventLog(opts.onEvent);
    const tmpRoot = await fs.realpath(os.tmpdir());
    const profileDir = await fs.mkdtemp(path.join(tmpRoot, spec.prefix));
    const realPath = await fs.realpath(profileDir);
    log.emit("env.created", {
      name: null,
      lifetime: "ephemeral",
      adapter_id: spec.adapterId,
      path: realPath,
    });
    let envOverrides: Record<string, string>;
    try {
      envOverrides = await materializeProfile(realPath, spec, log);
    } catch (e) {
      await removeDir(realPath);
      throw e;
    }
    return new Environment({
      path: realPath,
      envOverrides,
      spec,
      log,
      kind: "ephemeral",
      name: null,
      registry: null,
      destroyed: false,
    });
  }

  static async createOrAttach(
    name: string,
    opts: PersistentOptions = {},
  ): Promise<Environment> {
    const spec = await resolveSpec(opts);
    const log = new EventLog(opts.onEvent);
    const registry = new Registry(opts.registryRoot ?? defaultRegistryRoot());
    const { envDir, metadata, created } = await registry.reserveOrGet(name, spec.adapterId);
    const profileDir = path.resolve(envDir, "profile");
    let envOverrides: Record<string, string>;
    if (created) {
      log.emit("env.created", {
        name,
        lifetime: "persistent",
        adapter_id: spec.adapterId,
        path: profileDir,
      });
      envOverrides = await materializeProfile(profileDir, spec, log);
      const updated = {
        ...metadata,
        env_overrides: envOverrides,
        credentials_loaded: Object.keys(spec.credentials).length > 0,
        credentials_loaded_at:
          Object.keys(spec.credentials).length > 0 ? new Date().toISOString() : null,
      };
      await registry.updateMetadata(envDir, updated);
      log.emit("registry.written", { path: path.join(envDir, "metadata.json") });
    } else {
      log.emit("env.attached", {
        name,
        adapter_id: spec.adapterId,
        path: profileDir,
      });
      log.emit("registry.read", { path: path.join(envDir, "metadata.json") });
      envOverrides =
        Object.keys(metadata.env_overrides).length > 0
          ? { ...metadata.env_overrides }
          : await materializeProfile(profileDir, spec, log, { skipSeedIfExists: true });
    }
    return new Environment({
      path: profileDir,
      envOverrides,
      spec,
      log,
      kind: "persistent",
      name,
      registry,
      destroyed: false,
    });
  }

  static async attach(name: string, opts: PersistentOptions = {}): Promise<Environment> {
    const log = new EventLog(opts.onEvent);
    const registry = new Registry(opts.registryRoot ?? defaultRegistryRoot());
    const result = await registry.lookup(name);
    if (!result) {
      throw new EnvironmentNotFoundError(`no environment named '${name}'`, {
        name,
        registry_root: registry.root,
      });
    }
    const { envDir, metadata } = result;
    if (opts.adapter && opts.adapter.id !== metadata.adapter_id) {
      throw new InvalidEnvironmentSpecError(
        `env '${name}' was created with adapter '${metadata.adapter_id}' but '${opts.adapter.id}' was passed to attach`,
        {
          name,
          expected_adapter_id: metadata.adapter_id,
          actual_adapter_id: opts.adapter.id,
        },
      );
    }
    const profileDir = path.resolve(envDir, "profile");
    log.emit("env.attached", {
      name,
      adapter_id: metadata.adapter_id,
      path: profileDir,
    });
    log.emit("registry.read", { path: path.join(envDir, "metadata.json") });
    return new Environment({
      path: profileDir,
      envOverrides: { ...metadata.env_overrides },
      spec: new EnvironmentSpec({ adapterId: metadata.adapter_id, envOverrides: metadata.env_overrides }),
      log,
      kind: "persistent",
      name,
      registry,
      destroyed: false,
    });
  }

  static async list(opts: { registryRoot?: string } = {}): Promise<string[]> {
    return new Registry(opts.registryRoot ?? defaultRegistryRoot()).listNames();
  }

  static async destroyByName(name: string, opts: { registryRoot?: string } = {}): Promise<boolean> {
    const registry = new Registry(opts.registryRoot ?? defaultRegistryRoot());
    const { ok, error } = await registry.remove(name);
    if (!ok && error) {
      throw new CleanupFailedError(error, { path: registry.root, os_error: error });
    }
    return ok;
  }

  // ------------------------------------------------------------------
  // Instance API
  // ------------------------------------------------------------------

  get path(): string {
    return this.state.path;
  }

  get envOverrides(): Record<string, string> {
    return { ...this.state.envOverrides };
  }

  get adapterId(): string {
    return this.state.spec.adapterId;
  }

  get name(): string | null {
    return this.state.name;
  }

  get kind(): "ephemeral" | "persistent" {
    return this.state.kind;
  }

  events(): Event[] {
    return this.state.log.all();
  }

  async destroy(): Promise<boolean> {
    if (this.state.destroyed) return true;
    let ok = true;
    if (this.state.kind === "persistent") {
      if (!this.state.registry || !this.state.name) {
        throw new InternalInvariantViolationError("persistent env missing registry/name");
      }
      try {
        const { ok: cleanupOk, envDir, error } = await this.state.registry.remove(this.state.name);
        ok = cleanupOk;
        this.state.log.emit("registry.written", { path: this.state.registry.indexPath });
        this.state.log.emit("env.destroyed", { path: envDir ?? this.state.path, ok });
        if (!ok && error) {
          this.state.log.emit("error", { error_kind: "CleanupFailed", message: error });
        }
      } catch (e) {
        // already gone (EnvironmentNotFound) is fine
        const err = e as Error & { kind?: string };
        if (err.kind === "EnvironmentNotFound") {
          ok = true;
        } else {
          throw e;
        }
      }
    } else {
      ok = await removeDir(this.state.path);
      this.state.log.emit("env.destroyed", { path: this.state.path, ok });
      if (!ok) {
        this.state.log.emit("error", { error_kind: "CleanupFailed", message: "rmtree failed" });
      }
    }
    this.state.destroyed = true;
    return ok;
  }

  async refreshCredentials(spec?: EnvironmentSpec): Promise<void> {
    const s = spec ?? this.state.spec;
    if (Object.keys(s.credentials).length === 0) return;
    try {
      const { count } = await writeCredentialsOnly(this.state.path, s);
      this.state.log.emit("credentials.refreshed", { file_count: count });
    } catch (e) {
      throw new ProfileSetupFailedError(`refreshing credentials: ${(e as Error).message}`, {
        reason: "write_failed",
      });
    }
  }

  async [Symbol.asyncDispose](): Promise<void> {
    if (this.state.kind === "ephemeral") {
      await this.destroy();
    }
  }
}

async function resolveSpec(opts: EnvironmentOptions): Promise<EnvironmentSpec> {
  if (opts.spec && opts.adapter) {
    throw new InvalidEnvironmentSpecError(
      "pass either spec or adapter, not both",
      { field: "spec/adapter" },
    );
  }
  if (opts.adapter) {
    return await opts.adapter.buildSpec();
  }
  return opts.spec ?? new EnvironmentSpec();
}

async function writeCredentialsOnly(
  base: string,
  spec: EnvironmentSpec,
): Promise<{ count: number }> {
  let count = 0;
  for (const [rel, content] of Object.entries(spec.credentials)) {
    if (path.isAbsolute(rel)) {
      throw new ProfileSetupFailedError(`path must be relative: ${rel}`, { reason: "absolute_path" });
    }
    const target = path.resolve(base, rel);
    if (!target.startsWith(base + path.sep) && target !== base) {
      throw new ProfileSetupFailedError(`path escapes profile: ${rel}`, { reason: "resolved_escape" });
    }
    await fs.mkdir(path.dirname(target), { recursive: true });
    await fs.writeFile(target, content, { encoding: "utf-8" });
    const mode = spec.fileModes[rel] ?? 0o600;
    await fs.chmod(target, mode);
    count += 1;
  }
  return { count };
}
