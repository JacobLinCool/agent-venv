use crate::error::{Error, Result};
use crate::spec::EnvironmentSpec;

pub trait AgentAdapter {
    fn id(&self) -> &str;
    fn cli_bin(&self) -> &str;
    fn config_env_var(&self) -> &str;

    fn ensure_available(&self) -> Result<()> {
        if which::which(self.cli_bin()).is_err() {
            return Err(Error::AdapterUnavailable {
                adapter_id: self.id().to_string(),
                cli_bin: self.cli_bin().to_string(),
            });
        }
        Ok(())
    }

    fn build_spec(&self) -> Result<EnvironmentSpec>;

    fn build_argv(&self, _prompt: &str, _workspace: Option<&std::path::Path>) -> Vec<String> {
        Vec::new()
    }
}
