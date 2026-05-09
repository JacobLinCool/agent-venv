//! `agent-venv`: virtualenv-style profile isolation for coding-agent CLIs.
//!
//! See the spec/ directory at the repo root for the cross-language contract
//! this crate implements.

pub mod adapters;
mod environment;
mod error;
mod events;
mod profile;
mod registry;
mod spec;

pub use environment::Environment;
pub use error::{Error, Result as IsoResult};
pub use events::{Event, EventKind, EventSink};
pub use registry::{default_registry_root, Registry};
pub use spec::EnvironmentSpec;

pub const SPEC_VERSION: &str = "0.1";
pub const VERSION: &str = env!("CARGO_PKG_VERSION");
