import { execFile } from "node:child_process";
import { promises as fs } from "node:fs";
import * as os from "node:os";
import * as path from "node:path";
import { promisify } from "node:util";
import { ProfileSetupFailedError } from "../errors.js";

const execFileP = promisify(execFile);

export async function readClaudeCredentials(): Promise<string> {
  if (process.platform === "darwin") {
    try {
      const { stdout } = await execFileP("security", [
        "find-generic-password",
        "-s",
        "Claude Code-credentials",
        "-a",
        process.env["USER"] ?? "",
        "-w",
      ], { timeout: 10_000 });
      const trimmed = stdout.trim();
      if (trimmed.length > 0) {
        return trimmed.endsWith("\n") ? trimmed : trimmed + "\n";
      }
    } catch {
      // fall through
    }
  }
  const fallback = path.join(os.homedir(), ".claude", ".credentials.json");
  try {
    return await fs.readFile(fallback, "utf-8");
  } catch {
    throw new ProfileSetupFailedError(
      "Claude Code credentials not found in macOS Keychain ('Claude Code-credentials') or ~/.claude/.credentials.json. Run `claude` once to log in before running.",
      { reason: "claude_credentials_missing" },
    );
  }
}

export async function readCodexAuth(): Promise<string> {
  const src = path.join(os.homedir(), ".codex", "auth.json");
  try {
    return await fs.readFile(src, "utf-8");
  } catch {
    throw new ProfileSetupFailedError(
      "~/.codex/auth.json not found. Run `codex` once to log in before running.",
      { reason: "codex_auth_missing", path: src },
    );
  }
}
