use thiserror::Error;

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Error)]
pub enum Error {
    #[error("EnvironmentNotFound: {name}")]
    EnvironmentNotFound { name: String, registry_root: String },

    #[error("EnvironmentAlreadyExists: {name}")]
    EnvironmentAlreadyExists { name: String },

    #[error(
        "AdapterMismatch: env '{name}' was created with adapter '{expected_adapter_id}' but '{actual_adapter_id}' was requested"
    )]
    AdapterMismatch {
        name: String,
        expected_adapter_id: String,
        actual_adapter_id: String,
    },

    #[error("ProfileSetupFailed: {reason}")]
    ProfileSetupFailed { reason: String },

    #[error("RegistryUnavailable: {reason}")]
    RegistryUnavailable { reason: String, path: String },

    #[error("CredentialsMissing: {reason}")]
    CredentialsMissing { reason: String, adapter_id: String },

    #[error("AdapterUnavailable: '{cli_bin}' not found on PATH for adapter '{adapter_id}'")]
    AdapterUnavailable {
        adapter_id: String,
        cli_bin: String,
    },

    #[error("CleanupFailed: {os_error}")]
    CleanupFailed { os_error: String, path: String },

    #[error("InvalidEnvironmentSpec: {field}: {reason}")]
    InvalidEnvironmentSpec { field: String, reason: String },

    #[error("InternalInvariantViolation: {message}")]
    InternalInvariantViolation { message: String },
}

impl Error {
    pub fn kind(&self) -> &'static str {
        match self {
            Error::EnvironmentNotFound { .. } => "EnvironmentNotFound",
            Error::EnvironmentAlreadyExists { .. } => "EnvironmentAlreadyExists",
            Error::AdapterMismatch { .. } => "AdapterMismatch",
            Error::ProfileSetupFailed { .. } => "ProfileSetupFailed",
            Error::RegistryUnavailable { .. } => "RegistryUnavailable",
            Error::CredentialsMissing { .. } => "CredentialsMissing",
            Error::AdapterUnavailable { .. } => "AdapterUnavailable",
            Error::CleanupFailed { .. } => "CleanupFailed",
            Error::InvalidEnvironmentSpec { .. } => "InvalidEnvironmentSpec",
            Error::InternalInvariantViolation { .. } => "InternalInvariantViolation",
        }
    }
}
