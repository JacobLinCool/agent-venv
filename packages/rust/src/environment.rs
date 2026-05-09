use crate::error::{Error, Result};
use crate::events::{Event, EventKind, EventLog, EventSink};
use crate::profile::{materialize, remove_dir};
use crate::registry::{default_registry_root, Metadata, Registry};
use crate::spec::EnvironmentSpec;
use chrono::Utc;
use serde_json::{Map, Value};
use std::collections::HashMap;
use std::path::{Path, PathBuf};
use std::sync::Arc;

pub struct Environment {
    path: PathBuf,
    env_overrides: HashMap<String, String>,
    spec: EnvironmentSpec,
    log: Arc<EventLog>,
    kind: EnvKind,
    name: Option<String>,
    registry: Option<Registry>,
    destroyed: bool,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum EnvKind {
    Ephemeral,
    Persistent,
}

impl EnvKind {
    pub fn as_str(&self) -> &'static str {
        match self {
            EnvKind::Ephemeral => "ephemeral",
            EnvKind::Persistent => "persistent",
        }
    }
}

impl Environment {
    pub fn path(&self) -> &Path {
        &self.path
    }
    pub fn env_overrides(&self) -> &HashMap<String, String> {
        &self.env_overrides
    }
    pub fn adapter_id(&self) -> &str {
        &self.spec.adapter_id
    }
    pub fn name(&self) -> Option<&str> {
        self.name.as_deref()
    }
    pub fn kind(&self) -> EnvKind {
        self.kind
    }
    pub fn events(&self) -> Vec<Event> {
        self.log.snapshot()
    }

    // ------------------------------------------------------------------
    // Ephemeral
    // ------------------------------------------------------------------

    pub async fn ephemeral(spec: EnvironmentSpec) -> Result<Self> {
        Self::ephemeral_with_sink(spec, None).await
    }

    pub async fn ephemeral_with_sink(
        spec: EnvironmentSpec,
        on_event: Option<EventSink>,
    ) -> Result<Self> {
        let log = Arc::new(EventLog::new(on_event));
        let tmp_root = tokio::fs::canonicalize(std::env::temp_dir())
            .await
            .map_err(|e| Error::ProfileSetupFailed {
                reason: format!("canonicalize tmpdir: {e}"),
            })?;
        let raw = tempfile::Builder::new()
            .prefix(&spec.prefix)
            .tempdir_in(&tmp_root)
            .map_err(|e| Error::ProfileSetupFailed {
                reason: format!("mkdtemp: {e}"),
            })?
            .keep();
        let path = tokio::fs::canonicalize(&raw).await.unwrap_or(raw);

        let mut created = Map::new();
        created.insert("name".into(), Value::Null);
        created.insert("lifetime".into(), Value::from("ephemeral"));
        created.insert("adapter_id".into(), Value::from(spec.adapter_id.clone()));
        created.insert("path".into(), Value::from(path.to_string_lossy().to_string()));
        log.emit(EventKind::EnvCreated, created);

        match materialize(&path, &spec, &log, false).await {
            Ok(env_overrides) => Ok(Self {
                path,
                env_overrides,
                spec,
                log,
                kind: EnvKind::Ephemeral,
                name: None,
                registry: None,
                destroyed: false,
            }),
            Err(e) => {
                let mut m = Map::new();
                m.insert("error_kind".into(), Value::from(e.kind()));
                m.insert("message".into(), Value::from(e.to_string()));
                log.emit(EventKind::Error, m);
                let _ = remove_dir(&path).await;
                Err(e)
            }
        }
    }

    // ------------------------------------------------------------------
    // Persistent
    // ------------------------------------------------------------------

    pub async fn create_or_attach(
        name: &str,
        spec: EnvironmentSpec,
        registry_root: Option<&Path>,
    ) -> Result<Self> {
        Self::create_or_attach_with_sink(name, spec, registry_root, None).await
    }

    pub async fn create_or_attach_with_sink(
        name: &str,
        spec: EnvironmentSpec,
        registry_root: Option<&Path>,
        on_event: Option<EventSink>,
    ) -> Result<Self> {
        let log = Arc::new(EventLog::new(on_event));
        let registry = Registry::new(
            registry_root
                .map(Path::to_path_buf)
                .unwrap_or_else(default_registry_root),
        );
        let (env_dir, mut meta, created) = registry.reserve_or_get(name, &spec.adapter_id)?;
        let profile_dir = env_dir.join("profile");
        let env_overrides = if created {
            let mut m = Map::new();
            m.insert("name".into(), Value::from(name));
            m.insert("lifetime".into(), Value::from("persistent"));
            m.insert("adapter_id".into(), Value::from(spec.adapter_id.clone()));
            m.insert(
                "path".into(),
                Value::from(profile_dir.to_string_lossy().to_string()),
            );
            log.emit(EventKind::EnvCreated, m);
            let env_overrides = materialize(&profile_dir, &spec, &log, false).await?;
            meta.env_overrides = env_overrides.clone();
            meta.credentials_loaded = !spec.credentials.is_empty();
            if meta.credentials_loaded {
                meta.credentials_loaded_at = Some(Utc::now().to_rfc3339());
            }
            registry.update_metadata(&env_dir, &meta)?;
            let mut wm = Map::new();
            wm.insert(
                "path".into(),
                Value::from(env_dir.join("metadata.json").to_string_lossy().to_string()),
            );
            log.emit(EventKind::RegistryWritten, wm);
            env_overrides
        } else {
            let mut m = Map::new();
            m.insert("name".into(), Value::from(name));
            m.insert("adapter_id".into(), Value::from(spec.adapter_id.clone()));
            m.insert(
                "path".into(),
                Value::from(profile_dir.to_string_lossy().to_string()),
            );
            log.emit(EventKind::EnvAttached, m);
            let mut rm = Map::new();
            rm.insert(
                "path".into(),
                Value::from(env_dir.join("metadata.json").to_string_lossy().to_string()),
            );
            log.emit(EventKind::RegistryRead, rm);
            if !meta.env_overrides.is_empty() {
                meta.env_overrides.clone()
            } else {
                materialize(&profile_dir, &spec, &log, true).await?
            }
        };

        Ok(Self {
            path: profile_dir,
            env_overrides,
            spec,
            log,
            kind: EnvKind::Persistent,
            name: Some(name.to_string()),
            registry: Some(registry),
            destroyed: false,
        })
    }

    pub async fn attach(name: &str, registry_root: Option<&Path>) -> Result<Self> {
        let log = Arc::new(EventLog::new(None));
        let registry = Registry::new(
            registry_root
                .map(Path::to_path_buf)
                .unwrap_or_else(default_registry_root),
        );
        let (env_dir, meta) = registry
            .lookup(name)?
            .ok_or_else(|| Error::EnvironmentNotFound {
                name: name.to_string(),
                registry_root: registry.root.display().to_string(),
            })?;
        let profile_dir = env_dir.join("profile");
        let mut m = Map::new();
        m.insert("name".into(), Value::from(name));
        m.insert("adapter_id".into(), Value::from(meta.adapter_id.clone()));
        m.insert(
            "path".into(),
            Value::from(profile_dir.to_string_lossy().to_string()),
        );
        log.emit(EventKind::EnvAttached, m);
        let mut rm = Map::new();
        rm.insert(
            "path".into(),
            Value::from(env_dir.join("metadata.json").to_string_lossy().to_string()),
        );
        log.emit(EventKind::RegistryRead, rm);
        let spec = EnvironmentSpec {
            adapter_id: meta.adapter_id.clone(),
            env_overrides: meta.env_overrides.clone(),
            ..Default::default()
        };
        Ok(Self {
            path: profile_dir,
            env_overrides: meta.env_overrides.clone(),
            spec,
            log,
            kind: EnvKind::Persistent,
            name: Some(name.to_string()),
            registry: Some(registry),
            destroyed: false,
        })
    }

    pub fn list(registry_root: Option<&Path>) -> Result<Vec<String>> {
        let r = Registry::new(
            registry_root
                .map(Path::to_path_buf)
                .unwrap_or_else(default_registry_root),
        );
        r.list_names()
    }

    pub fn destroy_by_name(name: &str, registry_root: Option<&Path>) -> Result<bool> {
        let r = Registry::new(
            registry_root
                .map(Path::to_path_buf)
                .unwrap_or_else(default_registry_root),
        );
        let (ok, _, err) = r.remove(name)?;
        if !ok {
            if let Some(msg) = err {
                return Err(Error::CleanupFailed {
                    os_error: msg,
                    path: r.root.display().to_string(),
                });
            }
        }
        Ok(ok)
    }

    pub async fn destroy(&mut self) -> Result<bool> {
        if self.destroyed {
            return Ok(true);
        }
        let mut ok = true;
        if self.kind == EnvKind::Persistent {
            let registry = self.registry.as_ref().ok_or_else(|| {
                Error::InternalInvariantViolation {
                    message: "persistent env missing registry".into(),
                }
            })?;
            let name = self.name.as_ref().ok_or_else(|| {
                Error::InternalInvariantViolation {
                    message: "persistent env missing name".into(),
                }
            })?;
            match registry.remove(name) {
                Ok((cleanup_ok, env_dir, err)) => {
                    ok = cleanup_ok;
                    let mut wm = Map::new();
                    wm.insert(
                        "path".into(),
                        Value::from(registry.index_path.to_string_lossy().to_string()),
                    );
                    self.log.emit(EventKind::RegistryWritten, wm);
                    let mut dm = Map::new();
                    dm.insert(
                        "path".into(),
                        Value::from(env_dir.to_string_lossy().to_string()),
                    );
                    dm.insert("ok".into(), Value::from(ok));
                    self.log.emit(EventKind::EnvDestroyed, dm);
                    if !ok {
                        let mut em = Map::new();
                        em.insert("error_kind".into(), Value::from("CleanupFailed"));
                        em.insert(
                            "message".into(),
                            Value::from(err.unwrap_or_default()),
                        );
                        self.log.emit(EventKind::Error, em);
                    }
                }
                Err(Error::EnvironmentNotFound { .. }) => {
                    ok = true;
                }
                Err(e) => return Err(e),
            }
        } else {
            ok = remove_dir(&self.path).await;
            let mut dm = Map::new();
            dm.insert(
                "path".into(),
                Value::from(self.path.to_string_lossy().to_string()),
            );
            dm.insert("ok".into(), Value::from(ok));
            self.log.emit(EventKind::EnvDestroyed, dm);
            if !ok {
                let mut em = Map::new();
                em.insert("error_kind".into(), Value::from("CleanupFailed"));
                em.insert("message".into(), Value::from("rmtree failed"));
                self.log.emit(EventKind::Error, em);
            }
        }
        self.destroyed = true;
        Ok(ok)
    }
}

impl Drop for Environment {
    fn drop(&mut self) {
        if self.destroyed {
            return;
        }
        // Best-effort sync cleanup. We can't await in Drop; persistent envs are
        // intentionally NOT cleaned up here (their lifetime exceeds the handle).
        if self.kind == EnvKind::Ephemeral {
            let _ = std::fs::remove_dir_all(&self.path);
        }
        self.destroyed = true;
    }
}
