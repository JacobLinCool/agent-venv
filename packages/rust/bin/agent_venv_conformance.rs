use agent_venv::{Environment, EnvironmentSpec, Error, SPEC_VERSION, VERSION};
use serde_json::{json, Map, Value};
use std::io::{self, BufRead, Write};
use std::path::{Path, PathBuf};

fn spec_from_wire(p: &Value) -> EnvironmentSpec {
    let mut spec = EnvironmentSpec::default();
    if let Some(o) = p.as_object() {
        if let Some(s) = o.get("adapter_id").and_then(Value::as_str) {
            spec.adapter_id = s.to_string();
        }
        if let Some(eo) = o.get("env_overrides").and_then(Value::as_object) {
            for (k, v) in eo {
                if let Some(s) = v.as_str() {
                    spec.env_overrides.insert(k.clone(), s.to_string());
                }
            }
        }
        if let Some(sf) = o.get("seed_files").and_then(Value::as_object) {
            for (k, v) in sf {
                if let Some(s) = v.as_str() {
                    spec.seed_files.insert(k.clone(), s.to_string());
                }
            }
        }
        if let Some(cr) = o.get("credentials").and_then(Value::as_object) {
            for (k, v) in cr {
                if let Some(s) = v.as_str() {
                    spec.credentials.insert(k.clone(), s.to_string());
                }
            }
        }
        if let Some(fm) = o.get("file_modes").and_then(Value::as_object) {
            for (k, v) in fm {
                if let Some(n) = v.as_u64() {
                    spec.file_modes.insert(k.clone(), n as u32);
                }
            }
        }
        if let Some(s) = o.get("prefix").and_then(Value::as_str) {
            spec.prefix = s.to_string();
        }
    }
    spec
}

fn walk_files(base: &Path) -> Vec<String> {
    let mut out = Vec::new();
    fn recurse(dir: &Path, rel: &Path, out: &mut Vec<String>) {
        let entries = match std::fs::read_dir(dir) {
            Ok(e) => e,
            Err(_) => return,
        };
        for entry in entries.flatten() {
            let p = entry.path();
            let name = entry.file_name();
            let sub_rel = rel.join(&name);
            if p.is_dir() {
                recurse(&p, &sub_rel, out);
            } else if p.is_file() {
                out.push(sub_rel.to_string_lossy().to_string());
            }
        }
    }
    recurse(base, Path::new(""), &mut out);
    out.sort();
    out
}

fn inspect(env: &Environment) -> Map<String, Value> {
    let exists = env.path().exists();
    let files = if exists {
        walk_files(env.path())
    } else {
        Vec::new()
    };
    let mut file_modes_json = Map::new();
    #[cfg(unix)]
    if exists {
        use std::os::unix::fs::PermissionsExt;
        for rel in &files {
            if let Ok(meta) = std::fs::metadata(env.path().join(rel)) {
                let mode = meta.permissions().mode() & 0o777;
                file_modes_json.insert(rel.clone(), Value::from(mode));
            }
        }
    }
    let mut env_overrides = Map::new();
    for (k, v) in env.env_overrides() {
        env_overrides.insert(k.clone(), Value::from(v.clone()));
    }
    let mut m = Map::new();
    m.insert(
        "path".into(),
        Value::from(env.path().to_string_lossy().to_string()),
    );
    m.insert("exists".into(), Value::from(exists));
    m.insert("env_overrides".into(), Value::Object(env_overrides));
    m.insert(
        "files_present".into(),
        Value::Array(files.into_iter().map(Value::from).collect()),
    );
    m.insert("file_modes".into(), Value::Object(file_modes_json));
    m
}

fn events_to_wire(env: &Environment) -> Vec<Value> {
    env.events()
        .into_iter()
        .map(|e| Value::Object(e.to_wire()))
        .collect()
}

fn error_response(case_id: &str, kind: &str, message: &str) -> Value {
    json!({
        "case_id": case_id,
        "ok": true,
        "events": [],
        "error": {"kind": kind, "message": message},
    })
}

fn registry_root_from(req: &Value) -> Option<PathBuf> {
    req.get("registry_root")
        .and_then(Value::as_str)
        .map(PathBuf::from)
}

async fn handle(req: Value) -> Value {
    let case_id = req
        .get("case_id")
        .and_then(Value::as_str)
        .unwrap_or("")
        .to_string();
    let op = req
        .get("op")
        .and_then(Value::as_str)
        .unwrap_or("")
        .to_string();
    match op.as_str() {
        "ephemeral_lifecycle" => op_ephemeral(case_id, req).await,
        "persistent_create_attach_idempotent" => op_create_attach(case_id, req).await,
        "persistent_attach_only" => op_attach_only(case_id, req).await,
        "persistent_attach_missing" => op_attach_missing(case_id, req).await,
        "persistent_list" => op_list(case_id, req).await,
        "persistent_destroy_by_name" => op_destroy_by_name(case_id, req).await,
        "persistent_attach_mismatch" => op_attach_mismatch(case_id, req).await,
        _ => json!({
            "case_id": case_id,
            "ok": false,
            "error": {"kind": "InternalInvariantViolation", "message": format!("unknown op {op}")},
        }),
    }
}

async fn op_ephemeral(case_id: String, req: Value) -> Value {
    let spec = spec_from_wire(req.get("spec").unwrap_or(&Value::Null));
    let mut env_opt: Option<Environment> = None;
    let mut error: Option<(String, String)> = None;
    let mut inspection_obj = Map::new();
    match Environment::ephemeral(spec).await {
        Ok(e) => {
            inspection_obj = inspect(&e);
            env_opt = Some(e);
        }
        Err(e) => error = Some((e.kind().to_string(), e.to_string())),
    }
    let mut after_destroy = Map::new();
    let events = if let Some(mut env) = env_opt {
        let _ = env.destroy().await;
        after_destroy.insert("path_exists".into(), Value::from(env.path().exists()));
        events_to_wire(&env)
    } else {
        Vec::new()
    };
    let mut response = Map::new();
    response.insert("case_id".into(), Value::from(case_id));
    response.insert("ok".into(), Value::from(true));
    response.insert("events".into(), Value::Array(events));
    response.insert("inspection".into(), Value::Object(inspection_obj));
    response.insert("after_destroy".into(), Value::Object(after_destroy));
    if let Some((k, m)) = error {
        response.insert("error".into(), json!({"kind": k, "message": m}));
    }
    Value::Object(response)
}

async fn op_create_attach(case_id: String, req: Value) -> Value {
    let name = req
        .get("name")
        .and_then(Value::as_str)
        .unwrap_or("")
        .to_string();
    let registry_root = registry_root_from(&req);
    let spec = spec_from_wire(req.get("spec").unwrap_or(&Value::Null));
    let res = async {
        let env1 =
            Environment::create_or_attach(&name, spec.clone(), registry_root.as_deref()).await?;
        let env2 = Environment::create_or_attach(&name, spec, registry_root.as_deref()).await?;
        Ok::<_, Error>((env1, env2))
    }
    .await;
    match res {
        Ok((env1, env2)) => {
            let inspection2 = inspect(&env2);
            let mut events = events_to_wire(&env1);
            events.extend(events_to_wire(&env2));
            json!({
                "case_id": case_id,
                "ok": true,
                "events": events,
                "paths": [env1.path().to_string_lossy(), env2.path().to_string_lossy()],
                "second_path_files_present": inspection2.get("files_present").cloned().unwrap_or(Value::Array(vec![])),
            })
        }
        Err(e) => error_response(&case_id, e.kind(), &e.to_string()),
    }
}

async fn op_attach_only(case_id: String, req: Value) -> Value {
    let name = req
        .get("name")
        .and_then(Value::as_str)
        .unwrap_or("")
        .to_string();
    let registry_root = registry_root_from(&req);
    match Environment::attach(&name, registry_root.as_deref()).await {
        Ok(env) => {
            let inspection = inspect(&env);
            json!({
                "case_id": case_id,
                "ok": true,
                "events": events_to_wire(&env),
                "path": env.path().to_string_lossy(),
                "files_present": inspection.get("files_present").cloned().unwrap_or(Value::Array(vec![])),
            })
        }
        Err(e) => error_response(&case_id, e.kind(), &e.to_string()),
    }
}

async fn op_attach_missing(case_id: String, req: Value) -> Value {
    let name = req
        .get("name")
        .and_then(Value::as_str)
        .unwrap_or("")
        .to_string();
    let registry_root = registry_root_from(&req);
    match Environment::attach(&name, registry_root.as_deref()).await {
        Ok(_) => error_response(
            &case_id,
            "InternalInvariantViolation",
            "attach unexpectedly succeeded",
        ),
        Err(e) => error_response(&case_id, e.kind(), &e.to_string()),
    }
}

async fn op_list(case_id: String, req: Value) -> Value {
    let names: Vec<String> = req
        .get("names")
        .and_then(Value::as_array)
        .map(|a| {
            a.iter()
                .filter_map(|v| v.as_str().map(String::from))
                .collect()
        })
        .unwrap_or_default();
    let registry_root = registry_root_from(&req);
    let spec = spec_from_wire(req.get("spec").unwrap_or(&Value::Null));
    for n in &names {
        let _ = Environment::create_or_attach(n, spec.clone(), registry_root.as_deref()).await;
    }
    let listed = Environment::list(registry_root.as_deref()).unwrap_or_default();
    json!({
        "case_id": case_id,
        "ok": true,
        "events": [],
        "names_listed": listed,
    })
}

async fn op_destroy_by_name(case_id: String, req: Value) -> Value {
    let name = req
        .get("name")
        .and_then(Value::as_str)
        .unwrap_or("")
        .to_string();
    let registry_root = registry_root_from(&req);
    let spec = spec_from_wire(req.get("spec").unwrap_or(&Value::Null));
    let mut env = match Environment::create_or_attach(&name, spec, registry_root.as_deref()).await {
        Ok(e) => e,
        Err(e) => return error_response(&case_id, e.kind(), &e.to_string()),
    };
    let created_path = env.path().to_path_buf();
    let _ = env.destroy().await;
    let listed = Environment::list(registry_root.as_deref()).unwrap_or_default();
    json!({
        "case_id": case_id,
        "ok": true,
        "events": events_to_wire(&env),
        "created_path": created_path.to_string_lossy(),
        "path_exists_after": created_path.exists(),
        "name_in_index_after": listed.contains(&name),
    })
}

async fn op_attach_mismatch(case_id: String, req: Value) -> Value {
    let name = req
        .get("name")
        .and_then(Value::as_str)
        .unwrap_or("")
        .to_string();
    let registry_root = registry_root_from(&req);
    let first_spec = spec_from_wire(req.get("first_spec").unwrap_or(&Value::Null));
    let second_id = req
        .get("second_adapter_id")
        .and_then(Value::as_str)
        .unwrap_or("")
        .to_string();
    if let Err(e) = Environment::create_or_attach(&name, first_spec, registry_root.as_deref()).await
    {
        return error_response(&case_id, e.kind(), &e.to_string());
    }
    let second = EnvironmentSpec {
        adapter_id: second_id,
        ..EnvironmentSpec::default()
    };
    match Environment::create_or_attach(&name, second, registry_root.as_deref()).await {
        Ok(_) => error_response(
            &case_id,
            "InternalInvariantViolation",
            "expected AdapterMismatch but did not raise",
        ),
        Err(e) => error_response(&case_id, e.kind(), &e.to_string()),
    }
}

#[tokio::main(flavor = "multi_thread", worker_threads = 2)]
async fn main() -> io::Result<()> {
    let banner = json!({
        "protocol": "agent-venv.conformance",
        "version": 2,
        "language": "rust",
        "package_version": VERSION,
        "spec_version": SPEC_VERSION,
    });
    let stdout = io::stdout();
    {
        let mut out = stdout.lock();
        out.write_all(serde_json::to_string(&banner)?.as_bytes())?;
        out.write_all(b"\n")?;
        out.flush()?;
    }
    let stdin = io::stdin();
    let mut lines = stdin.lock().lines();
    while let Some(Ok(line)) = lines.next() {
        let line = line.trim();
        if line.is_empty() {
            continue;
        }
        let req: Value = match serde_json::from_str(line) {
            Ok(v) => v,
            Err(e) => {
                let resp = json!({
                    "ok": false,
                    "error": {"kind": "InternalInvariantViolation", "message": format!("bad request: {e}")},
                });
                let mut out = stdout.lock();
                out.write_all(serde_json::to_string(&resp)?.as_bytes())?;
                out.write_all(b"\n")?;
                out.flush()?;
                continue;
            }
        };
        let resp = handle(req).await;
        let mut out = stdout.lock();
        out.write_all(serde_json::to_string(&resp)?.as_bytes())?;
        out.write_all(b"\n")?;
        out.flush()?;
    }
    Ok(())
}
