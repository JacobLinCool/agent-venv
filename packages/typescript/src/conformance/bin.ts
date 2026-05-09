import { promises as fs, constants as fsConstants, Dirent } from "node:fs";
import * as path from "node:path";
import * as readline from "node:readline";
import { Environment } from "../environment.js";
import { EnvironmentSpec } from "../spec.js";
import { eventToWire } from "../events.js";
import { AgentVenvError } from "../errors.js";
import { SPEC_VERSION, VERSION } from "../version.js";

interface InspectionResult {
  path: string;
  exists: boolean;
  env_overrides: Record<string, string>;
  files_present: string[];
  file_modes: Record<string, number>;
}

async function walkFiles(base: string): Promise<string[]> {
  const results: string[] = [];
  async function recurse(dir: string, rel: string) {
    let entries: Dirent[];
    try {
      entries = await fs.readdir(dir, { withFileTypes: true });
    } catch {
      return;
    }
    for (const entry of entries) {
      const sub = path.join(dir, entry.name);
      const subRel = rel ? path.join(rel, entry.name) : entry.name;
      if (entry.isDirectory()) {
        await recurse(sub, subRel);
      } else if (entry.isFile()) {
        results.push(subRel);
      }
    }
  }
  await recurse(base, "");
  results.sort();
  return results;
}

async function inspect(env: Environment): Promise<InspectionResult> {
  const exists = await fs
    .access(env.path, fsConstants.F_OK)
    .then(() => true)
    .catch(() => false);
  let files: string[] = [];
  const fileModes: Record<string, number> = {};
  if (exists) {
    files = await walkFiles(env.path);
    if (process.platform !== "win32") {
      for (const rel of files) {
        try {
          const stat = await fs.stat(path.join(env.path, rel));
          fileModes[rel] = stat.mode & 0o777;
        } catch {
          // skip
        }
      }
    }
  }
  return {
    path: env.path,
    exists,
    env_overrides: env.envOverrides,
    files_present: files,
    file_modes: fileModes,
  };
}

function eventsToWire(env: Environment) {
  return env.events().map(eventToWire);
}

interface RawRequest {
  case_id?: string;
  op?: string;
  spec?: Record<string, unknown>;
  first_spec?: Record<string, unknown>;
  second_adapter_id?: string;
  name?: string;
  names?: string[];
  registry_root?: string;
}

async function handleEphemeralLifecycle(req: RawRequest): Promise<Record<string, unknown>> {
  const spec = EnvironmentSpec.fromWire(req.spec);
  let env: Environment | null = null;
  let inspection: InspectionResult | Record<string, unknown> = {};
  let afterDestroy: Record<string, unknown> = {};
  let error: { kind: string; message: string } | null = null;
  try {
    env = await Environment.ephemeral({ spec });
    inspection = await inspect(env);
  } catch (e) {
    if (e instanceof AgentVenvError) {
      error = { kind: e.kind, message: e.message };
    } else {
      throw e;
    }
  } finally {
    if (env) {
      await env.destroy();
      const exists = await fs
        .access(env.path, fsConstants.F_OK)
        .then(() => true)
        .catch(() => false);
      afterDestroy = { path_exists: exists };
    }
  }
  const response: Record<string, unknown> = {
    case_id: req.case_id ?? "",
    ok: true,
    events: env ? eventsToWire(env) : [],
    inspection,
    after_destroy: afterDestroy,
  };
  if (error) response["error"] = error;
  return response;
}

async function handleCreateAttachIdempotent(req: RawRequest): Promise<Record<string, unknown>> {
  const name = req.name ?? "";
  const registryRoot = req.registry_root ?? "";
  const spec = EnvironmentSpec.fromWire(req.spec);
  const env1 = await Environment.createOrAttach(name, { spec, registryRoot });
  const env2 = await Environment.createOrAttach(name, { spec, registryRoot });
  const inspection2 = await inspect(env2);
  return {
    case_id: req.case_id ?? "",
    ok: true,
    events: [...eventsToWire(env1), ...eventsToWire(env2)],
    paths: [env1.path, env2.path],
    second_path_files_present: inspection2.files_present,
  };
}

async function handleAttachOnly(req: RawRequest): Promise<Record<string, unknown>> {
  const name = req.name ?? "";
  const registryRoot = req.registry_root ?? "";
  const env = await Environment.attach(name, { registryRoot });
  const inspection = await inspect(env);
  return {
    case_id: req.case_id ?? "",
    ok: true,
    events: eventsToWire(env),
    path: env.path,
    files_present: inspection.files_present,
  };
}

async function handleAttachMissing(req: RawRequest): Promise<Record<string, unknown>> {
  const name = req.name ?? "";
  const registryRoot = req.registry_root ?? "";
  try {
    const env = await Environment.attach(name, { registryRoot });
    return {
      case_id: req.case_id ?? "",
      ok: true,
      events: eventsToWire(env),
      error: { kind: "InternalInvariantViolation", message: "attach unexpectedly succeeded" },
    };
  } catch (e) {
    if (e instanceof AgentVenvError) {
      return {
        case_id: req.case_id ?? "",
        ok: true,
        events: [],
        error: { kind: e.kind, message: e.message },
      };
    }
    throw e;
  }
}

async function handleList(req: RawRequest): Promise<Record<string, unknown>> {
  const names = req.names ?? [];
  const registryRoot = req.registry_root ?? "";
  const spec = EnvironmentSpec.fromWire(req.spec);
  for (const n of names) {
    await Environment.createOrAttach(n, { spec, registryRoot });
  }
  const listed = await Environment.list({ registryRoot });
  return {
    case_id: req.case_id ?? "",
    ok: true,
    events: [],
    names_listed: listed,
  };
}

async function handleDestroyByName(req: RawRequest): Promise<Record<string, unknown>> {
  const name = req.name ?? "";
  const registryRoot = req.registry_root ?? "";
  const spec = EnvironmentSpec.fromWire(req.spec);
  const env = await Environment.createOrAttach(name, { spec, registryRoot });
  const createdPath = env.path;
  await env.destroy();
  const listed = await Environment.list({ registryRoot });
  const exists = await fs
    .access(createdPath, fsConstants.F_OK)
    .then(() => true)
    .catch(() => false);
  return {
    case_id: req.case_id ?? "",
    ok: true,
    events: eventsToWire(env),
    created_path: createdPath,
    path_exists_after: exists,
    name_in_index_after: listed.includes(name),
  };
}

async function handleAttachMismatch(req: RawRequest): Promise<Record<string, unknown>> {
  const name = req.name ?? "";
  const registryRoot = req.registry_root ?? "";
  const firstSpec = EnvironmentSpec.fromWire(req.first_spec);
  const secondAdapterId = req.second_adapter_id ?? "";
  await Environment.createOrAttach(name, { spec: firstSpec, registryRoot });
  const secondSpec = new EnvironmentSpec({ adapterId: secondAdapterId });
  try {
    await Environment.createOrAttach(name, { spec: secondSpec, registryRoot });
    return {
      case_id: req.case_id ?? "",
      ok: true,
      events: [],
      error: {
        kind: "InternalInvariantViolation",
        message: "expected AdapterMismatch but did not raise",
      },
    };
  } catch (e) {
    if (e instanceof AgentVenvError) {
      return {
        case_id: req.case_id ?? "",
        ok: true,
        events: [],
        error: { kind: e.kind, message: e.message },
      };
    }
    throw e;
  }
}

async function dispatch(req: RawRequest): Promise<Record<string, unknown>> {
  const op = req.op;
  switch (op) {
    case "ephemeral_lifecycle":
      return handleEphemeralLifecycle(req);
    case "persistent_create_attach_idempotent":
      return handleCreateAttachIdempotent(req);
    case "persistent_attach_only":
      return handleAttachOnly(req);
    case "persistent_attach_missing":
      return handleAttachMissing(req);
    case "persistent_list":
      return handleList(req);
    case "persistent_destroy_by_name":
      return handleDestroyByName(req);
    case "persistent_attach_mismatch":
      return handleAttachMismatch(req);
    default:
      return {
        case_id: req.case_id ?? "",
        ok: false,
        error: {
          kind: "InternalInvariantViolation",
          message: `unknown op ${op}`,
        },
      };
  }
}

async function main(): Promise<void> {
  process.stdout.write(
    JSON.stringify({
      protocol: "agent-venv.conformance",
      version: 2,
      language: "typescript",
      package_version: VERSION,
      spec_version: SPEC_VERSION,
    }) + "\n",
  );

  const rl = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
  for await (const rawLine of rl) {
    const line = rawLine.trim();
    if (!line) continue;
    let req: RawRequest;
    try {
      req = JSON.parse(line);
    } catch (e) {
      process.stdout.write(
        JSON.stringify({
          ok: false,
          error: {
            kind: "InternalInvariantViolation",
            message: `bad request: ${(e as Error).message}`,
          },
        }) + "\n",
      );
      continue;
    }
    let response: Record<string, unknown>;
    try {
      response = await dispatch(req);
    } catch (e) {
      response = {
        case_id: req.case_id ?? "",
        ok: false,
        error: {
          kind: "InternalInvariantViolation",
          message: (e as Error).stack ?? (e as Error).message,
        },
      };
    }
    process.stdout.write(JSON.stringify(response) + "\n");
  }
}

main().catch((e) => {
  process.stderr.write(`conformance bin crashed: ${(e as Error).stack ?? e}\n`);
  process.exit(1);
});
