import { promises as fs } from "node:fs";
import * as path from "node:path";
import { ProfileSetupFailedError } from "./errors.js";
import type { EventLog } from "./events.js";
import type { EnvironmentSpec } from "./spec.js";

async function writeFiles(
  base: string,
  files: Record<string, string>,
  fileModes: Record<string, number>,
  defaultMode?: number,
): Promise<{ count: number; total: number }> {
  let count = 0;
  let total = 0;
  for (const [rel, content] of Object.entries(files)) {
    if (path.isAbsolute(rel)) {
      throw new ProfileSetupFailedError(`path must be relative: ${rel}`, {
        reason: "absolute_path",
      });
    }
    for (const part of rel.split(path.sep)) {
      if (part === "..") {
        throw new ProfileSetupFailedError(`path escapes profile: ${rel}`, {
          reason: "parent_traversal",
        });
      }
    }
    const target = path.resolve(base, rel);
    if (!target.startsWith(base + path.sep) && target !== base) {
      throw new ProfileSetupFailedError(`path escapes profile: ${rel}`, {
        reason: "resolved_escape",
      });
    }
    await fs.mkdir(path.dirname(target), { recursive: true });
    const buffer = Buffer.from(content, "utf-8");
    await fs.writeFile(target, buffer);
    const mode = fileModes[rel] ?? defaultMode;
    if (mode !== undefined) {
      await fs.chmod(target, mode);
    }
    count += 1;
    total += buffer.length;
  }
  return { count, total };
}

export async function materializeProfile(
  profileDir: string,
  spec: EnvironmentSpec,
  log: EventLog,
  options: { skipSeedIfExists?: boolean } = {},
): Promise<Record<string, string>> {
  const skipSeed = options.skipSeedIfExists ?? false;
  await fs.mkdir(profileDir, { recursive: true });
  const homeStr = await fs.realpath(profileDir).catch(() => profileDir);

  if (!skipSeed && Object.keys(spec.seedFiles).length > 0) {
    const { count, total } = await writeFiles(profileDir, spec.seedFiles, spec.fileModes);
    log.emit("profile.materialized", { file_count: count, total_bytes: total });
  }
  if (!skipSeed && Object.keys(spec.credentials).length > 0) {
    const { count } = await writeFiles(profileDir, spec.credentials, spec.fileModes, 0o600);
    log.emit("credentials.copied", { file_count: count });
  }

  const envOverrides: Record<string, string> = {};
  for (const [k, v] of Object.entries(spec.envOverrides)) {
    envOverrides[k] = v.replace("$EPHEMERAL_HOME", homeStr);
  }
  return envOverrides;
}

export async function removeDir(p: string): Promise<boolean> {
  try {
    await fs.rm(p, { recursive: true, force: true });
    return true;
  } catch {
    return false;
  }
}
