import { EnvironmentSpec } from "../spec.js";
import { AgentAdapter, ensureBinAvailable } from "./base.js";
import { readCodexAuth } from "./credentials.js";

export interface CodexOptions {
  model?: string;
  reasoningEffort?: string;
  loadCredentials?: boolean;
  extraArgv?: string[];
}

export function Codex(opts: CodexOptions = {}): AgentAdapter {
  const loadCredentials = opts.loadCredentials ?? true;
  const adapter: AgentAdapter = {
    id: "codex",
    cliBin: "codex",
    configEnvVar: "CODEX_HOME",
    async ensureAvailable() {
      await ensureBinAvailable(this.id, this.cliBin);
    },
    async buildSpec(): Promise<EnvironmentSpec> {
      const credentials: Record<string, string> = {};
      const fileModes: Record<string, number> = {};
      if (loadCredentials) {
        credentials["auth.json"] = await readCodexAuth();
        fileModes["auth.json"] = 0o600;
      }
      return new EnvironmentSpec({
        adapterId: "codex",
        envOverrides: { CODEX_HOME: "$EPHEMERAL_HOME" },
        credentials,
        fileModes,
        prefix: "agent-venv-codex-",
      });
    },
    buildArgv(_prompt: string, options: { workspace?: string } = {}): string[] {
      if (!opts.model) {
        throw new Error("Codex.buildArgv requires options.model");
      }
      const argv = ["codex", "exec", "--model", opts.model];
      if (options.workspace) argv.push("--cd", options.workspace);
      argv.push("--json", "--skip-git-repo-check", "--dangerously-bypass-approvals-and-sandbox");
      if (opts.reasoningEffort) argv.push("-c", `model_reasoning_effort="${opts.reasoningEffort}"`);
      if (opts.extraArgv) argv.push(...opts.extraArgv);
      return argv;
    },
  };
  return adapter;
}
