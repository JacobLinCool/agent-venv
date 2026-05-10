use agent_venv::adapters::{AgentAdapter, ClaudeCode, Codex};

#[test]
fn claude_spec_no_creds() {
    let spec = ClaudeCode::new()
        .load_credentials(false)
        .build_spec()
        .unwrap();
    assert_eq!(spec.adapter_id, "claude-code");
    assert_eq!(
        spec.env_overrides.get("CLAUDE_CONFIG_DIR"),
        Some(&"$EPHEMERAL_HOME".to_string())
    );
    assert!(spec.seed_files.contains_key(".claude.json"));
    assert!(!spec.credentials.contains_key(".credentials.json"));
}

#[test]
fn codex_spec_no_creds() {
    let spec = Codex::new().load_credentials(false).build_spec().unwrap();
    assert_eq!(spec.adapter_id, "codex");
    assert_eq!(
        spec.env_overrides.get("CODEX_HOME"),
        Some(&"$EPHEMERAL_HOME".to_string())
    );
}

#[test]
fn claude_argv() {
    let argv = ClaudeCode::new()
        .model("claude-haiku-4-5-20251001")
        .reasoning_effort("high")
        .build_argv("hi", None);
    assert_eq!(argv[0], "claude");
    assert!(argv.contains(&"--print".to_string()));
    assert!(argv.contains(&"claude-haiku-4-5-20251001".to_string()));
    assert!(argv.contains(&"high".to_string()));
}

#[test]
fn codex_argv_with_workspace() {
    let argv = Codex::new()
        .model("gpt-5")
        .build_argv("hi", Some(std::path::Path::new("/tmp/ws")));
    assert_eq!(argv[0], "codex");
    assert!(argv.contains(&"exec".to_string()));
    assert!(argv.contains(&"gpt-5".to_string()));
    assert!(argv.contains(&"/tmp/ws".to_string()));
}
