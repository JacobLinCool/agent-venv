import { promises as fs, constants as fsConstants } from "node:fs";
import { createHash } from "node:crypto";
import * as os from "node:os";
import * as path from "node:path";
import {
  AdapterMismatchError,
  EnvironmentNotFoundError,
  RegistryUnavailableError,
} from "./errors.js";

export const REGISTRY_SCHEMA_VERSION = 1;

export function defaultRegistryRoot(): string {
  const override = process.env["AGENT_VENV_REGISTRY_ROOT"];
  if (override) return override;
  const xdg = process.env["XDG_DATA_HOME"];
  if (xdg) return path.join(xdg, "agent-venv", "envs");
  return path.join(os.homedir(), ".local", "share", "agent-venv", "envs");
}

export function slugFor(name: string): string {
  return createHash("sha256").update(name).digest("hex").slice(0, 16);
}

export interface Metadata {
  schema_version: number;
  name: string;
  adapter_id: string;
  created_at: string;
  env_overrides: Record<string, string>;
  credentials_loaded: boolean;
  credentials_loaded_at: string | null;
}

export class Registry {
  readonly root: string;
  readonly indexPath: string;
  readonly envsDir: string;
  readonly lockPath: string;

  constructor(root: string) {
    this.root = path.resolve(root);
    this.indexPath = path.join(this.root, "index.json");
    this.envsDir = path.join(this.root, "envs");
    this.lockPath = path.join(this.root, ".lock");
  }

  // ------------------------------------------------------------------
  // Locking — Node has no native flock; use exclusive O_EXCL lockfile
  // with retry. This is good enough for v0 advisory cross-process
  // serialization within a single host.
  // ------------------------------------------------------------------

  private async acquireLock(): Promise<() => Promise<void>> {
    await fs.mkdir(this.root, { recursive: true });
    for (let i = 0; i < 100; i++) {
      try {
        const fh = await fs.open(this.lockPath, fsConstants.O_CREAT | fsConstants.O_EXCL | fsConstants.O_RDWR, 0o644);
        await fh.close();
        return async () => {
          try {
            await fs.unlink(this.lockPath);
          } catch {
            // best-effort
          }
        };
      } catch (e: unknown) {
        const code = (e as NodeJS.ErrnoException).code;
        if (code !== "EEXIST") {
          throw new RegistryUnavailableError(`could not acquire registry lock: ${(e as Error).message}`, {
            path: this.lockPath,
          });
        }
        await new Promise((r) => setTimeout(r, 50));
      }
    }
    throw new RegistryUnavailableError("could not acquire registry lock", { path: this.lockPath });
  }

  // ------------------------------------------------------------------
  // Index
  // ------------------------------------------------------------------

  private async readIndex(): Promise<Record<string, string>> {
    try {
      const raw = await fs.readFile(this.indexPath, "utf-8");
      const data = JSON.parse(raw) as { entries?: Record<string, string> };
      return { ...(data.entries ?? {}) };
    } catch (e: unknown) {
      const code = (e as NodeJS.ErrnoException).code;
      if (code === "ENOENT") return {};
      throw new RegistryUnavailableError(`reading index: ${(e as Error).message}`, {
        path: this.indexPath,
      });
    }
  }

  private async writeIndex(entries: Record<string, string>): Promise<void> {
    await fs.mkdir(this.root, { recursive: true });
    const payload = { schema_version: REGISTRY_SCHEMA_VERSION, entries };
    const tmp = `${this.indexPath}.tmp`;
    await fs.writeFile(tmp, JSON.stringify(payload, null, 2), { encoding: "utf-8" });
    await fs.rename(tmp, this.indexPath);
  }

  // ------------------------------------------------------------------
  // Public ops
  // ------------------------------------------------------------------

  async listNames(): Promise<string[]> {
    return Object.keys(await this.readIndex()).sort();
  }

  async lookup(name: string): Promise<{ envDir: string; metadata: Metadata } | null> {
    const entries = await this.readIndex();
    const rel = entries[name];
    if (!rel) return null;
    let envDir = path.resolve(this.root, rel);
    try {
      envDir = await fs.realpath(envDir);
    } catch {
      // dir might not exist if state is corrupted; keep the resolved path
    }
    const metaPath = path.join(envDir, "metadata.json");
    try {
      const raw = await fs.readFile(metaPath, "utf-8");
      const metadata = JSON.parse(raw) as Metadata;
      return { envDir, metadata };
    } catch {
      return null;
    }
  }

  async reserveOrGet(
    name: string,
    adapterId: string,
  ): Promise<{ envDir: string; metadata: Metadata; created: boolean }> {
    const release = await this.acquireLock();
    try {
      const entries = await this.readIndex();
      if (entries[name]) {
        let envDir = path.resolve(this.root, entries[name]);
        try {
          envDir = await fs.realpath(envDir);
        } catch {
          // keep resolved path
        }
        const metaPath = path.join(envDir, "metadata.json");
        const raw = await fs.readFile(metaPath, "utf-8");
        const metadata = JSON.parse(raw) as Metadata;
        if (metadata.adapter_id !== adapterId) {
          throw new AdapterMismatchError(
            `env '${name}' was created with adapter '${metadata.adapter_id}' but '${adapterId}' was requested`,
            {
              name,
              expected_adapter_id: metadata.adapter_id,
              actual_adapter_id: adapterId,
            },
          );
        }
        return { envDir, metadata, created: false };
      }
      let slug = slugFor(name);
      const existingSlugs = new Set(Object.values(entries).map((p) => path.basename(p)));
      let attempt = slug;
      let i = 0;
      while (existingSlugs.has(attempt)) {
        i += 1;
        attempt = `${slug}-${i}`;
      }
      slug = attempt;
      const relDir = `envs/${slug}`;
      let envDir = path.resolve(this.root, relDir);
      await fs.mkdir(path.join(envDir, "profile"), { recursive: true });
      envDir = await fs.realpath(envDir);
      const metadata: Metadata = {
        schema_version: REGISTRY_SCHEMA_VERSION,
        name,
        adapter_id: adapterId,
        created_at: new Date().toISOString(),
        env_overrides: {},
        credentials_loaded: false,
        credentials_loaded_at: null,
      };
      await fs.writeFile(path.join(envDir, "metadata.json"), JSON.stringify(metadata, null, 2));
      entries[name] = relDir;
      await this.writeIndex(entries);
      return { envDir, metadata, created: true };
    } finally {
      await release();
    }
  }

  async updateMetadata(envDir: string, metadata: Metadata): Promise<void> {
    try {
      await fs.writeFile(path.join(envDir, "metadata.json"), JSON.stringify(metadata, null, 2));
    } catch (e: unknown) {
      throw new RegistryUnavailableError(`writing metadata: ${(e as Error).message}`, {
        path: path.join(envDir, "metadata.json"),
      });
    }
  }

  async remove(name: string): Promise<{ ok: boolean; envDir: string | null; error: string | null }> {
    const release = await this.acquireLock();
    try {
      const entries = await this.readIndex();
      if (!entries[name]) {
        throw new EnvironmentNotFoundError(`no environment named '${name}'`, {
          name,
          registry_root: this.root,
        });
      }
      const rel = entries[name]!;
      delete entries[name];
      const envDir = path.resolve(this.root, rel);
      let cleanupErr: string | null = null;
      try {
        await fs.rm(envDir, { recursive: true, force: true });
      } catch (e: unknown) {
        cleanupErr = (e as Error).message;
      }
      await this.writeIndex(entries);
      return { ok: cleanupErr === null, envDir, error: cleanupErr };
    } finally {
      await release();
    }
  }
}
