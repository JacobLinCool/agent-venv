use agent_venv::{Environment, EnvironmentSpec, Error};

#[tokio::test]
async fn ephemeral_create_destroy() {
    let spec = EnvironmentSpec::builder()
        .env_override("FOO", "$EPHEMERAL_HOME")
        .build();
    let mut env = Environment::ephemeral(spec).await.unwrap();
    let p = env.path().to_path_buf();
    assert!(p.exists());
    assert_eq!(env.env_overrides().get("FOO"), Some(&p.to_string_lossy().to_string()));
    env.destroy().await.unwrap();
    assert!(!p.exists());
}

#[tokio::test]
async fn ephemeral_seed_files() {
    let spec = EnvironmentSpec::builder()
        .seed_file("a.txt", "hi")
        .seed_file("nested/b.txt", "yo")
        .build();
    let mut env = Environment::ephemeral(spec).await.unwrap();
    let a = std::fs::read_to_string(env.path().join("a.txt")).unwrap();
    assert_eq!(a, "hi");
    env.destroy().await.unwrap();
}

#[tokio::test]
async fn ephemeral_no_home_in_env_overrides() {
    let spec = EnvironmentSpec::builder()
        .env_override("CLAUDE_CONFIG_DIR", "$EPHEMERAL_HOME")
        .build();
    let mut env = Environment::ephemeral(spec).await.unwrap();
    assert!(env.env_overrides().contains_key("CLAUDE_CONFIG_DIR"));
    assert!(!env.env_overrides().contains_key("HOME"));
    env.destroy().await.unwrap();
}

#[tokio::test]
async fn credentials_default_mode_0600() {
    use std::os::unix::fs::PermissionsExt;
    let spec = EnvironmentSpec::builder()
        .credential(".credentials.json", "{\"k\":\"v\"}")
        .build();
    let mut env = Environment::ephemeral(spec).await.unwrap();
    let mode = std::fs::metadata(env.path().join(".credentials.json"))
        .unwrap()
        .permissions()
        .mode()
        & 0o777;
    assert_eq!(mode, 0o600);
    env.destroy().await.unwrap();
}

#[tokio::test]
async fn persistent_create_or_attach_idempotent() {
    let tmp = tempfile::tempdir().unwrap();
    let spec = EnvironmentSpec::builder().seed_file("x.txt", "1").build();
    let mut e1 = Environment::create_or_attach("E", spec.clone(), Some(tmp.path()))
        .await
        .unwrap();
    let mut e2 = Environment::create_or_attach("E", spec, Some(tmp.path()))
        .await
        .unwrap();
    assert_eq!(e1.path(), e2.path());
    let _ = e1.destroy().await; // also removes registry entry
    let _ = e2.destroy().await; // idempotent: already gone
}

#[tokio::test]
async fn attach_missing_raises() {
    let tmp = tempfile::tempdir().unwrap();
    match Environment::attach("nope", Some(tmp.path())).await {
        Err(Error::EnvironmentNotFound { .. }) => {}
        Err(other) => panic!("expected EnvironmentNotFound, got {:?}", other),
        Ok(_) => panic!("expected EnvironmentNotFound, got Ok"),
    }
}

#[tokio::test]
async fn list_and_destroy_by_name() {
    let tmp = tempfile::tempdir().unwrap();
    let spec = EnvironmentSpec::default();
    Environment::create_or_attach("a", spec.clone(), Some(tmp.path()))
        .await
        .unwrap();
    Environment::create_or_attach("b", spec, Some(tmp.path()))
        .await
        .unwrap();
    let mut names = Environment::list(Some(tmp.path())).unwrap();
    names.sort();
    assert_eq!(names, vec!["a", "b"]);
    Environment::destroy_by_name("a", Some(tmp.path())).unwrap();
    assert_eq!(Environment::list(Some(tmp.path())).unwrap(), vec!["b"]);
}

#[tokio::test]
async fn attach_mismatch() {
    let tmp = tempfile::tempdir().unwrap();
    let mut a = EnvironmentSpec::default();
    a.adapter_id = "claude-code".into();
    let mut b = EnvironmentSpec::default();
    b.adapter_id = "codex".into();
    Environment::create_or_attach("multi", a, Some(tmp.path()))
        .await
        .unwrap();
    match Environment::create_or_attach("multi", b, Some(tmp.path())).await {
        Err(Error::AdapterMismatch { .. }) => {}
        Err(other) => panic!("expected AdapterMismatch, got {:?}", other),
        Ok(_) => panic!("expected AdapterMismatch, got Ok"),
    }
}
