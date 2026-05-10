use crate::error::Error;
use std::path::PathBuf;
#[cfg(target_os = "macos")]
use std::process::Command;

pub(crate) fn read_claude_credentials() -> Result<String, Error> {
    #[cfg(target_os = "macos")]
    {
        let output = Command::new("security")
            .args([
                "find-generic-password",
                "-s",
                "Claude Code-credentials",
                "-a",
                std::env::var("USER").unwrap_or_default().as_str(),
                "-w",
            ])
            .output();
        if let Ok(out) = output {
            if out.status.success() {
                let s = String::from_utf8_lossy(&out.stdout).trim().to_string();
                if !s.is_empty() {
                    return Ok(if s.ends_with('\n') {
                        s
                    } else {
                        format!("{s}\n")
                    });
                }
            }
        }
    }
    let home = home_dir();
    let fallback = home.join(".claude").join(".credentials.json");
    std::fs::read_to_string(&fallback).map_err(|_| Error::CredentialsMissing {
        reason: "Claude Code credentials not found in macOS Keychain ('Claude Code-credentials') or ~/.claude/.credentials.json".into(),
        adapter_id: "claude-code".into(),
    })
}

pub(crate) fn read_codex_auth() -> Result<String, Error> {
    let home = home_dir();
    let src = home.join(".codex").join("auth.json");
    std::fs::read_to_string(&src).map_err(|_| Error::CredentialsMissing {
        reason: format!("{} not found. Run `codex` once to log in.", src.display()),
        adapter_id: "codex".into(),
    })
}

fn home_dir() -> PathBuf {
    std::env::var_os("HOME")
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from("/"))
}
