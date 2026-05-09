import { promises as fs } from "node:fs";
import * as path from "node:path";
import { AdapterUnavailableError } from "../errors.js";
import { EnvironmentSpec } from "../spec.js";

export interface AgentAdapter {
  readonly id: string;
  readonly cliBin: string;
  readonly configEnvVar: string;
  ensureAvailable(): Promise<void>;
  buildSpec(): Promise<EnvironmentSpec> | EnvironmentSpec;
  buildArgv?(prompt: string, opts?: { workspace?: string }): string[];
}

export async function whichBin(name: string): Promise<string | null> {
  if (path.isAbsolute(name)) {
    try {
      await fs.access(name, fs.constants.X_OK);
      return name;
    } catch {
      return null;
    }
  }
  const PATH = process.env["PATH"] ?? "";
  const sep = process.platform === "win32" ? ";" : ":";
  for (const dir of PATH.split(sep)) {
    if (!dir) continue;
    const candidate = path.join(dir, name);
    try {
      await fs.access(candidate, fs.constants.X_OK);
      return candidate;
    } catch {
      // continue
    }
  }
  return null;
}

export async function ensureBinAvailable(adapterId: string, cliBin: string): Promise<void> {
  const found = await whichBin(cliBin);
  if (!found) {
    throw new AdapterUnavailableError(
      `'${cliBin}' not found on PATH for adapter '${adapterId}'`,
      { adapter_id: adapterId, cli_bin: cliBin },
    );
  }
}
