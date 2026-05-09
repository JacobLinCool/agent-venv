export interface EnvironmentSpecInput {
  adapterId?: string;
  envOverrides?: Record<string, string>;
  seedFiles?: Record<string, string>;
  fileModes?: Record<string, number>;
  credentials?: Record<string, string>;
  prefix?: string;
}

export class EnvironmentSpec {
  readonly adapterId: string;
  readonly envOverrides: Record<string, string>;
  readonly seedFiles: Record<string, string>;
  readonly fileModes: Record<string, number>;
  readonly credentials: Record<string, string>;
  readonly prefix: string;

  constructor(input: EnvironmentSpecInput = {}) {
    this.adapterId = input.adapterId ?? "generic";
    this.envOverrides = { ...(input.envOverrides ?? {}) };
    this.seedFiles = { ...(input.seedFiles ?? {}) };
    this.fileModes = { ...(input.fileModes ?? {}) };
    this.credentials = { ...(input.credentials ?? {}) };
    this.prefix = input.prefix ?? "agent-venv-";
  }

  static fromWire(payload: Record<string, unknown> | undefined): EnvironmentSpec {
    const p = payload ?? {};
    return new EnvironmentSpec({
      adapterId: typeof p["adapter_id"] === "string" ? (p["adapter_id"] as string) : "generic",
      envOverrides: (p["env_overrides"] as Record<string, string>) ?? {},
      seedFiles: (p["seed_files"] as Record<string, string>) ?? {},
      fileModes: (p["file_modes"] as Record<string, number>) ?? {},
      credentials: (p["credentials"] as Record<string, string>) ?? {},
      prefix: typeof p["prefix"] === "string" ? (p["prefix"] as string) : "agent-venv-",
    });
  }
}
