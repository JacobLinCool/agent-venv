# agent-venv (Rust)

Rust implementation of [`agent-venv`](https://github.com/JacobLinCool/agent-venv). See the root README for the project overview and the [`spec/`](../../spec) directory for the cross-language contract.

## Install

```toml
[dependencies]
agent-venv = "0.1"
tokio = { version = "1", features = ["macros", "rt-multi-thread"] }
```

MSRV: 1.75. Async-only (tokio).

## Layer 1: any CLI

```rust
use agent_venv::{Sandbox, Policy};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let mut sb = Sandbox::builder()
        .policy(Policy::default().with_max_runtime_ms(30_000))
        .build()
        .await?;
    sb.seed([("main.rs", "fn main(){ println!(\"hi\"); }")]).await?;
    let out = sb.run(["sh", "-c", "echo hello"]).await?;
    println!("{}", out.stdout);
    sb.destroy().await?;
    Ok(())
}
```

## Layer 2: built-in adapters

```rust
use agent_venv::{Sandbox, adapters::ClaudeCode};

let mut sb = Sandbox::with_agent(ClaudeCode::new("claude-haiku-4-5-20251001"))
    .build()
    .await?;
let out = sb.run_agent("add a README").await?;
sb.destroy().await?;
```

## Cleanup

`Drop` does best-effort cleanup (logs a warning on failure but never panics). For deterministic error reporting, call `destroy().await?` explicitly.

## Errors

```rust
use agent_venv::{Error, Sandbox, Policy};

let mut sb = Sandbox::builder().policy(Policy::default().with_max_runtime_ms(100)).build().await?;
match sb.run(["sh", "-c", "sleep 5"]).await {
    Err(Error::Timeout { .. }) => println!("timed out"),
    other => println!("{:?}", other),
}
```

`Error::kind() -> &'static str` returns the spec-defined kind string.

## Conformance

```bash
cargo build --release
./target/release/agent-venv-conformance < requests.ndjson
```
