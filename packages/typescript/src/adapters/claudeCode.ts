import { EnvironmentSpec } from "../spec.js";
import { AgentAdapter, ensureBinAvailable } from "./base.js";
import { readClaudeCredentials } from "./credentials.js";

export interface ClaudeCodeOptions {
  model?: string;
  reasoningEffort?: string;
  loadCredentials?: boolean;
  extraArgv?: string[];
}

export function ClaudeCode(opts: ClaudeCodeOptions = {}): AgentAdapter {
  const loadCredentials = opts.loadCredentials ?? true;
  const adapter: AgentAdapter = {
    id: "claude-code",
    cliBin: "claude",
    configEnvVar: "CLAUDE_CONFIG_DIR",
    async ensureAvailable() {
      await ensureBinAvailable(this.id, this.cliBin);
    },
    async buildSpec(): Promise<EnvironmentSpec> {
      const seedFiles: Record<string, string> = {};
      const credentials: Record<string, string> = {};
      const fileModes: Record<string, number> = {};
      if (loadCredentials) {
        credentials[".credentials.json"] = await readClaudeCredentials();
        fileModes[".credentials.json"] = 0o600;
      } else {
        seedFiles[".claude.json"] = JSON.stringify({ hasCompletedOnboarding: true });
      }
      return new EnvironmentSpec({
        adapterId: "claude-code",
        envOverrides: { CLAUDE_CONFIG_DIR: "$EPHEMERAL_HOME" },
        seedFiles,
        credentials,
        fileModes,
        prefix: "agent-venv-claude-",
      });
    },
    buildArgv(_prompt: string): string[] {
      if (!opts.model) {
        throw new Error("ClaudeCode.buildArgv requires options.model");
      }
      const argv = [
        "claude",
        "--print",
        "--model",
        opts.model,
        "--output-format",
        "stream-json",
        "--verbose",
        "--dangerously-skip-permissions",
      ];
      if (opts.reasoningEffort) argv.push("--effort", opts.reasoningEffort);
      if (opts.extraArgv) argv.push(...opts.extraArgv);
      return argv;
    },
  };
  return adapter;
}
