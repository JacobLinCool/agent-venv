use std::collections::HashMap;

#[derive(Debug, Clone)]
pub struct EnvironmentSpec {
    pub adapter_id: String,
    pub env_overrides: HashMap<String, String>,
    pub seed_files: HashMap<String, String>,
    pub file_modes: HashMap<String, u32>,
    pub credentials: HashMap<String, String>,
    pub prefix: String,
}

impl Default for EnvironmentSpec {
    fn default() -> Self {
        Self {
            adapter_id: "generic".into(),
            env_overrides: HashMap::new(),
            seed_files: HashMap::new(),
            file_modes: HashMap::new(),
            credentials: HashMap::new(),
            prefix: "agent-venv-".into(),
        }
    }
}

impl EnvironmentSpec {
    pub fn builder() -> EnvironmentSpecBuilder {
        EnvironmentSpecBuilder::default()
    }
}

#[derive(Default)]
pub struct EnvironmentSpecBuilder {
    spec: EnvironmentSpec,
}

impl EnvironmentSpecBuilder {
    pub fn adapter_id(mut self, id: impl Into<String>) -> Self {
        self.spec.adapter_id = id.into();
        self
    }
    pub fn env_override(mut self, k: impl Into<String>, v: impl Into<String>) -> Self {
        self.spec.env_overrides.insert(k.into(), v.into());
        self
    }
    pub fn seed_file(mut self, rel: impl Into<String>, content: impl Into<String>) -> Self {
        self.spec.seed_files.insert(rel.into(), content.into());
        self
    }
    pub fn credential(mut self, rel: impl Into<String>, content: impl Into<String>) -> Self {
        self.spec.credentials.insert(rel.into(), content.into());
        self
    }
    pub fn file_mode(mut self, rel: impl Into<String>, mode: u32) -> Self {
        self.spec.file_modes.insert(rel.into(), mode);
        self
    }
    pub fn prefix(mut self, prefix: impl Into<String>) -> Self {
        self.spec.prefix = prefix.into();
        self
    }
    pub fn build(self) -> EnvironmentSpec {
        self.spec
    }
}
