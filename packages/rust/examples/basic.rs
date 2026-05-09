use agent_venv::{Environment, EnvironmentSpec};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let spec = EnvironmentSpec::builder()
        .env_override("FOO", "$EPHEMERAL_HOME")
        .seed_file("hello.txt", "world")
        .build();
    let mut env = Environment::ephemeral(spec).await?;
    println!("path: {}", env.path().display());
    println!("env_overrides: {:?}", env.env_overrides());
    env.destroy().await?;
    Ok(())
}
