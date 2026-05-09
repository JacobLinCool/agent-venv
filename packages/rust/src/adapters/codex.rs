use super::base::AgentAdapter;
use super::credentials::read_codex_auth;
use crate::error::Result;
use crate::spec::EnvironmentSpec;

pub struct Codex {
    pub model: Option<String>,
    pub reasoning_effort: Option<String>,
    pub extra_argv: Vec<String>,
    pub load_credentials: bool,
}

impl Codex {
    pub fn new() -> Self {
        Self {
            model: None,
            reasoning_effort: None,
            extra_argv: Vec::new(),
            load_credentials: true,
        }
    }
    pub fn model(mut self, m: impl Into<String>) -> Self {
        self.model = Some(m.into());
        self
    }
    pub fn load_credentials(mut self, yes: bool) -> Self {
        self.load_credentials = yes;
        self
    }
}

impl Default for Codex {
    fn default() -> Self {
        Self::new()
    }
}

impl AgentAdapter for Codex {
    fn id(&self) -> &str {
        "codex"
    }
    fn cli_bin(&self) -> &str {
        "codex"
    }
    fn config_env_var(&self) -> &str {
        "CODEX_HOME"
    }

    fn build_spec(&self) -> Result<EnvironmentSpec> {
        let mut builder = EnvironmentSpec::builder()
            .adapter_id("codex")
            .env_override("CODEX_HOME", "$EPHEMERAL_HOME")
            .prefix("agent-venv-codex-");
        if self.load_credentials {
            let auth = read_codex_auth()?;
            builder = builder
                .credential("auth.json", auth)
                .file_mode("auth.json", 0o600);
        }
        Ok(builder.build())
    }

    fn build_argv(&self, _prompt: &str, workspace: Option<&std::path::Path>) -> Vec<String> {
        let mut argv: Vec<String> = vec![
            "codex".into(),
            "exec".into(),
            "--model".into(),
            self.model.clone().unwrap_or_default(),
        ];
        if let Some(ws) = workspace {
            argv.push("--cd".into());
            argv.push(ws.to_string_lossy().to_string());
        }
        argv.push("--json".into());
        argv.push("--skip-git-repo-check".into());
        argv.push("--dangerously-bypass-approvals-and-sandbox".into());
        if let Some(e) = &self.reasoning_effort {
            argv.push("-c".into());
            argv.push(format!("model_reasoning_effort=\"{e}\""));
        }
        argv.extend(self.extra_argv.iter().cloned());
        argv
    }
}
