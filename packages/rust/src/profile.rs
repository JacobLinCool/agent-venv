use crate::error::{Error, Result};
use crate::events::{EventKind, EventLog};
use crate::spec::EnvironmentSpec;
use serde_json::{Map, Value};
use std::collections::HashMap;
use std::path::{Path, PathBuf};

pub(crate) async fn write_files(
    base: &Path,
    files: &HashMap<String, String>,
    file_modes: &HashMap<String, u32>,
    default_mode: Option<u32>,
) -> Result<(u64, u64)> {
    let base_canon = tokio::fs::canonicalize(base)
        .await
        .unwrap_or_else(|_| base.to_path_buf());
    let mut count = 0u64;
    let mut total = 0u64;
    for (rel, content) in files {
        let rel_path = Path::new(rel);
        if rel_path.is_absolute() {
            return Err(Error::ProfileSetupFailed {
                reason: format!("path must be relative: {rel}"),
            });
        }
        for comp in rel_path.components() {
            if !matches!(comp, std::path::Component::Normal(_)) {
                return Err(Error::ProfileSetupFailed {
                    reason: format!("path escapes profile: {rel}"),
                });
            }
        }
        let target = base_canon.join(rel_path);
        if let Some(parent) = target.parent() {
            tokio::fs::create_dir_all(parent)
                .await
                .map_err(|e| Error::ProfileSetupFailed {
                    reason: format!("mkdir {}: {e}", parent.display()),
                })?;
        }
        let bytes = content.as_bytes();
        tokio::fs::write(&target, bytes)
            .await
            .map_err(|e| Error::ProfileSetupFailed {
                reason: format!("write {}: {e}", target.display()),
            })?;
        let mode = file_modes.get(rel).copied().or(default_mode);
        if let Some(m) = mode {
            #[cfg(unix)]
            {
                use std::os::unix::fs::PermissionsExt;
                let perms = std::fs::Permissions::from_mode(m);
                tokio::fs::set_permissions(&target, perms)
                    .await
                    .map_err(|e| Error::ProfileSetupFailed {
                        reason: format!("chmod {}: {e}", target.display()),
                    })?;
            }
            #[cfg(not(unix))]
            {
                let _ = m;
            }
        }
        count += 1;
        total += bytes.len() as u64;
    }
    Ok((count, total))
}

pub(crate) async fn materialize(
    profile_dir: &Path,
    spec: &EnvironmentSpec,
    log: &EventLog,
    skip_seed_if_exists: bool,
) -> Result<HashMap<String, String>> {
    tokio::fs::create_dir_all(profile_dir)
        .await
        .map_err(|e| Error::ProfileSetupFailed {
            reason: format!("mkdir {}: {e}", profile_dir.display()),
        })?;
    let canon = tokio::fs::canonicalize(profile_dir)
        .await
        .unwrap_or_else(|_| profile_dir.to_path_buf());

    if !skip_seed_if_exists && !spec.seed_files.is_empty() {
        let (count, total) = write_files(&canon, &spec.seed_files, &spec.file_modes, None).await?;
        let mut m = Map::new();
        m.insert("file_count".into(), Value::from(count));
        m.insert("total_bytes".into(), Value::from(total));
        log.emit(EventKind::ProfileMaterialized, m);
    }
    if !skip_seed_if_exists && !spec.credentials.is_empty() {
        let (count, _) =
            write_files(&canon, &spec.credentials, &spec.file_modes, Some(0o600)).await?;
        let mut m = Map::new();
        m.insert("file_count".into(), Value::from(count));
        log.emit(EventKind::CredentialsCopied, m);
    }

    let home_str = canon.to_string_lossy().to_string();
    let mut env_overrides = HashMap::new();
    for (k, v) in &spec.env_overrides {
        env_overrides.insert(k.clone(), v.replace("$EPHEMERAL_HOME", &home_str));
    }
    Ok(env_overrides)
}

pub(crate) async fn remove_dir(path: &Path) -> bool {
    if !path.exists() {
        return true;
    }
    tokio::fs::remove_dir_all(path).await.is_ok()
}

pub(crate) fn _ensure_pathbuf(p: &Path) -> PathBuf {
    p.to_path_buf()
}
