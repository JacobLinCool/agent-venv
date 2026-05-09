use crate::error::{Error, Result};
use chrono::Utc;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use sha2::{Digest, Sha256};
use std::collections::HashMap;
use std::fs::{File, OpenOptions};
use std::io::ErrorKind as IoErrorKind;
use std::path::{Path, PathBuf};

pub const REGISTRY_SCHEMA_VERSION: u32 = 1;

pub fn default_registry_root() -> PathBuf {
    if let Ok(s) = std::env::var("AGENT_VENV_REGISTRY_ROOT") {
        return PathBuf::from(s);
    }
    if let Ok(s) = std::env::var("XDG_DATA_HOME") {
        return PathBuf::from(s).join("agent-venv").join("envs");
    }
    let home = std::env::var_os("HOME")
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from("/"));
    home.join(".local").join("share").join("agent-venv").join("envs")
}

pub fn slug_for(name: &str) -> String {
    let mut h = Sha256::new();
    h.update(name.as_bytes());
    let digest = h.finalize();
    hex::encode(&digest[..8])
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Metadata {
    pub schema_version: u32,
    pub name: String,
    pub adapter_id: String,
    pub created_at: String,
    #[serde(default)]
    pub env_overrides: HashMap<String, String>,
    #[serde(default)]
    pub credentials_loaded: bool,
    #[serde(default)]
    pub credentials_loaded_at: Option<String>,
}

pub struct Registry {
    pub root: PathBuf,
    pub index_path: PathBuf,
    pub envs_dir: PathBuf,
    pub lock_path: PathBuf,
}

impl Registry {
    pub fn new(root: impl Into<PathBuf>) -> Self {
        let root = root.into();
        let index_path = root.join("index.json");
        let envs_dir = root.join("envs");
        let lock_path = root.join(".lock");
        Self {
            root,
            index_path,
            envs_dir,
            lock_path,
        }
    }

    fn acquire_lock(&self) -> Result<File> {
        std::fs::create_dir_all(&self.root).map_err(|e| Error::RegistryUnavailable {
            reason: e.to_string(),
            path: self.root.display().to_string(),
        })?;
        for _ in 0..200 {
            match OpenOptions::new()
                .write(true)
                .create_new(true)
                .open(&self.lock_path)
            {
                Ok(f) => return Ok(f),
                Err(e) if e.kind() == IoErrorKind::AlreadyExists => {
                    std::thread::sleep(std::time::Duration::from_millis(25));
                }
                Err(e) => {
                    return Err(Error::RegistryUnavailable {
                        reason: e.to_string(),
                        path: self.lock_path.display().to_string(),
                    });
                }
            }
        }
        Err(Error::RegistryUnavailable {
            reason: "could not acquire lock".into(),
            path: self.lock_path.display().to_string(),
        })
    }

    fn release_lock(&self, _f: File) {
        let _ = std::fs::remove_file(&self.lock_path);
    }

    fn read_index(&self) -> Result<HashMap<String, String>> {
        match std::fs::read_to_string(&self.index_path) {
            Ok(s) => {
                let v: Value =
                    serde_json::from_str(&s).map_err(|e| Error::RegistryUnavailable {
                        reason: format!("parse index: {e}"),
                        path: self.index_path.display().to_string(),
                    })?;
                let entries = v.get("entries").and_then(Value::as_object);
                let mut out = HashMap::new();
                if let Some(o) = entries {
                    for (k, v) in o {
                        if let Some(s) = v.as_str() {
                            out.insert(k.clone(), s.to_string());
                        }
                    }
                }
                Ok(out)
            }
            Err(e) if e.kind() == IoErrorKind::NotFound => Ok(HashMap::new()),
            Err(e) => Err(Error::RegistryUnavailable {
                reason: e.to_string(),
                path: self.index_path.display().to_string(),
            }),
        }
    }

    fn write_index(&self, entries: &HashMap<String, String>) -> Result<()> {
        std::fs::create_dir_all(&self.root).map_err(|e| Error::RegistryUnavailable {
            reason: e.to_string(),
            path: self.root.display().to_string(),
        })?;
        let mut payload = serde_json::Map::new();
        payload.insert(
            "schema_version".into(),
            Value::from(REGISTRY_SCHEMA_VERSION),
        );
        let mut entries_obj = serde_json::Map::new();
        for (k, v) in entries {
            entries_obj.insert(k.clone(), Value::from(v.clone()));
        }
        payload.insert("entries".into(), Value::Object(entries_obj));
        let serialized = serde_json::to_string_pretty(&Value::Object(payload)).unwrap();
        let tmp = self.index_path.with_extension("json.tmp");
        std::fs::write(&tmp, serialized).map_err(|e| Error::RegistryUnavailable {
            reason: e.to_string(),
            path: tmp.display().to_string(),
        })?;
        std::fs::rename(&tmp, &self.index_path).map_err(|e| Error::RegistryUnavailable {
            reason: e.to_string(),
            path: self.index_path.display().to_string(),
        })?;
        Ok(())
    }

    pub fn list_names(&self) -> Result<Vec<String>> {
        let mut names: Vec<String> = self.read_index()?.into_keys().collect();
        names.sort();
        Ok(names)
    }

    pub fn lookup(&self, name: &str) -> Result<Option<(PathBuf, Metadata)>> {
        let entries = self.read_index()?;
        let rel = match entries.get(name) {
            Some(r) => r.clone(),
            None => return Ok(None),
        };
        let env_dir = std::fs::canonicalize(self.root.join(&rel))
            .unwrap_or_else(|_| self.root.join(&rel));
        let meta_path = env_dir.join("metadata.json");
        match std::fs::read_to_string(&meta_path) {
            Ok(s) => {
                let meta: Metadata =
                    serde_json::from_str(&s).map_err(|e| Error::RegistryUnavailable {
                        reason: format!("parse metadata: {e}"),
                        path: meta_path.display().to_string(),
                    })?;
                Ok(Some((env_dir, meta)))
            }
            Err(_) => Ok(None),
        }
    }

    pub fn reserve_or_get(
        &self,
        name: &str,
        adapter_id: &str,
    ) -> Result<(PathBuf, Metadata, bool)> {
        let lock = self.acquire_lock()?;
        let result = (|| -> Result<(PathBuf, Metadata, bool)> {
            let mut entries = self.read_index()?;
            if let Some(rel) = entries.get(name) {
                let env_dir = std::fs::canonicalize(self.root.join(rel))
                    .unwrap_or_else(|_| self.root.join(rel));
                let meta_path = env_dir.join("metadata.json");
                let meta: Metadata = serde_json::from_str(
                    &std::fs::read_to_string(&meta_path).map_err(|e| {
                        Error::RegistryUnavailable {
                            reason: e.to_string(),
                            path: meta_path.display().to_string(),
                        }
                    })?,
                )
                .map_err(|e| Error::RegistryUnavailable {
                    reason: format!("parse metadata: {e}"),
                    path: meta_path.display().to_string(),
                })?;
                if meta.adapter_id != adapter_id {
                    return Err(Error::AdapterMismatch {
                        name: name.to_string(),
                        expected_adapter_id: meta.adapter_id.clone(),
                        actual_adapter_id: adapter_id.to_string(),
                    });
                }
                return Ok((env_dir, meta, false));
            }
            let mut slug = slug_for(name);
            let existing: std::collections::HashSet<String> = entries
                .values()
                .map(|p| Path::new(p).file_name().unwrap().to_string_lossy().to_string())
                .collect();
            let mut attempt = slug.clone();
            let mut i = 0;
            while existing.contains(&attempt) {
                i += 1;
                attempt = format!("{slug}-{i}");
            }
            slug = attempt;
            let rel_dir = format!("envs/{slug}");
            let env_dir_initial = self.root.join(&rel_dir);
            std::fs::create_dir_all(env_dir_initial.join("profile")).map_err(|e| {
                Error::RegistryUnavailable {
                    reason: e.to_string(),
                    path: env_dir_initial.display().to_string(),
                }
            })?;
            let env_dir = std::fs::canonicalize(&env_dir_initial)
                .unwrap_or_else(|_| env_dir_initial.clone());
            let meta = Metadata {
                schema_version: REGISTRY_SCHEMA_VERSION,
                name: name.to_string(),
                adapter_id: adapter_id.to_string(),
                created_at: Utc::now().to_rfc3339(),
                env_overrides: HashMap::new(),
                credentials_loaded: false,
                credentials_loaded_at: None,
            };
            std::fs::write(
                env_dir.join("metadata.json"),
                serde_json::to_string_pretty(&meta).unwrap(),
            )
            .map_err(|e| Error::RegistryUnavailable {
                reason: e.to_string(),
                path: env_dir.join("metadata.json").display().to_string(),
            })?;
            entries.insert(name.to_string(), rel_dir);
            self.write_index(&entries)?;
            Ok((env_dir, meta, true))
        })();
        self.release_lock(lock);
        result
    }

    pub fn update_metadata(&self, env_dir: &Path, meta: &Metadata) -> Result<()> {
        let path = env_dir.join("metadata.json");
        std::fs::write(&path, serde_json::to_string_pretty(meta).unwrap()).map_err(|e| {
            Error::RegistryUnavailable {
                reason: e.to_string(),
                path: path.display().to_string(),
            }
        })
    }

    pub fn remove(&self, name: &str) -> Result<(bool, PathBuf, Option<String>)> {
        let lock = self.acquire_lock()?;
        let result = (|| -> Result<(bool, PathBuf, Option<String>)> {
            let mut entries = self.read_index()?;
            let rel = match entries.remove(name) {
                Some(r) => r,
                None => {
                    return Err(Error::EnvironmentNotFound {
                        name: name.to_string(),
                        registry_root: self.root.display().to_string(),
                    });
                }
            };
            let env_dir = self.root.join(&rel);
            let mut cleanup_err: Option<String> = None;
            if env_dir.exists() {
                if let Err(e) = std::fs::remove_dir_all(&env_dir) {
                    cleanup_err = Some(e.to_string());
                }
            }
            self.write_index(&entries)?;
            Ok((cleanup_err.is_none(), env_dir, cleanup_err))
        })();
        self.release_lock(lock);
        result
    }
}
