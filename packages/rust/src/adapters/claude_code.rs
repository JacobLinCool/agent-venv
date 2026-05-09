use super::base::AgentAdapter;
use super::credentials::read_claude_credentials;
use crate::error::Result;
use crate::spec::EnvironmentSpec;

pub struct ClaudeCode {
    pub model: Option<String>,
    pub reasoning_effort: Option<String>,
    pub extra_argv: Vec<String>,
    pub load_credentials: bool,
}

impl ClaudeCode {
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
    pub fn reasoning_effort(mut self, e: impl Into<String>) -> Self {
        self.reasoning_effort = Some(e.into());
        self
    }
}

impl Default for ClaudeCode {
    fn default() -> Self {
        Self::new()
    }
}

impl AgentAdapter for ClaudeCode {
    fn id(&self) -> &str {
        "claude-code"
    }
    fn cli_bin(&self) -> &str {
        "claude"
    }
    fn config_env_var(&self) -> &str {
        "CLAUDE_CONFIG_DIR"
    }

    fn build_spec(&self) -> Result<EnvironmentSpec> {
        let mut builder = EnvironmentSpec::builder()
            .adapter_id("claude-code")
            .env_override("CLAUDE_CONFIG_DIR", "$EPHEMERAL_HOME")
            .prefix("agent-venv-claude-");
        if self.load_credentials {
            let creds = read_claude_credentials()?;
            builder = builder
                .credential(".credentials.json", creds)
                .file_mode(".credentials.json", 0o600);
        } else {
            builder = builder.seed_file(".claude.json", "{\"hasCompletedOnboarding\":true}");
        }
        Ok(builder.build())
    }

    fn build_argv(&self, _prompt: &str, _workspace: Option<&std::path::Path>) -> Vec<String> {
        let mut argv: Vec<String> = vec![
            "claude".into(),
            "--print".into(),
            "--model".into(),
            self.model.clone().unwrap_or_default(),
            "--output-format".into(),
            "stream-json".into(),
            "--verbose".into(),
            "--dangerously-skip-permissions".into(),
        ];
        if let Some(e) = &self.reasoning_effort {
            argv.push("--effort".into());
            argv.push(e.clone());
        }
        argv.extend(self.extra_argv.iter().cloned());
        argv
    }
}
