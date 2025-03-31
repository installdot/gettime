package main
// ver2
import (
    "fmt"
    "io"
    "net"
    "runtime"
    "strings"
    "sync"
    "sync/atomic"
    "time"

    "github.com/valyala/fasthttp"
)

func main() {
    // Configuration
    const targetRPS = 300000
    const testDuration = 230 * time.Second
    url := "http://your-api-endpoint.com" // Replace with your API URL (HTTP/2 preferred)

    // Optimize runtime
    runtime.GOMAXPROCS(runtime.NumCPU())
    runtime.GC() // Force garbage collection upfront

    // Statistics
    var totalRequests int64
    var successfulRequests int64
    var failedRequests int64
    var peakRPS int64

    // Fetch proxies
    proxies := fetchProxies()
    if len(proxies) == 0 {
        fmt.Println("No proxies found, using direct connection.")
        proxies = append(proxies, "") // Direct connection fallback
    }
    fmt.Printf("Loaded %d proxies\n", len(proxies))

    // Create fasthttp client with HTTP/2 and proxy support
    client := &fasthttp.Client{
        MaxConnsPerHost:     20000,           // Extremely high connection limit
        ReadTimeout:         500 * time.Millisecond,
        WriteTimeout:        500 * time.Millisecond,
        MaxIdleConnDuration: 0, // Keep connections alive
        Dial: func(addr string) (net.Conn, error) {
            // Rotate proxies
            proxyIdx := int(atomic.AddInt64(&totalRequests, 1)) % len(proxies)
            if proxies[proxyIdx] == "" {
                return fasthttp.Dial(addr) // Direct connection
            }
            return fasthttp.DialDualStack(proxies[proxyIdx]) // Proxy connection
        },
    }

    // WaitGroup and done channel
    var wg sync.WaitGroup
    done := make(chan struct{})

    // RPS tracking
    rpsTicker := time.NewTicker(1 * time.Second)
    defer rpsTicker.Stop()
    var requestsThisSecond int64

    // Start request sender
    startTime := time.Now()
    workerCount := runtime.NumCPU() * 1000 // 1000x CPU cores for extreme concurrency
    for i := 0; i < workerCount; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            req := fasthttp.AcquireRequest()
            req.SetRequestURI(url)
            req.Header.SetMethod("GET")
            resp := fasthttp.AcquireResponse()
            defer fasthttp.ReleaseRequest(req)
            defer fasthttp.ReleaseResponse(resp)

            for {
                select {
                case <-done:
                    return
                default:
                    atomic.AddInt64(&requestsThisSecond, 1)
                    err := client.Do(req, resp)
                    if err != nil {
                        atomic.AddInt64(&failedRequests, 1)

                        continue
                    }
                    if resp.StatusCode() >= 200 && resp.StatusCode() < 300 {
                        atomic.AddInt64(&successfulRequests, 1)
                    } else {
                        atomic.AddInt64(&failedRequests, 1)
                    }
                }
            }
        }()
    }

    // Pace to target RPS (optional fallback)
    go func() {
        ticker := time.NewTicker(time.Second / targetRPS)
        defer ticker.Stop()
        for {
            select {
            case <-done:
                return
            case <-ticker.C:
                atomic.AddInt64(&totalRequests, 1)
            }
        }
    }()

    // Track peak RPS
    go func() {
        for {
            select {
            case <-done:
                return
            case <-rpsTicker.C:
                currentRPS := atomic.SwapInt64(&requestsThisSecond, 0)
                if currentRPS > peakRPS {
                    atomic.StoreInt64(&peakRPS, currentRPS)
                }
            }
        }
    }()

    // Run for 230 seconds
    time.Sleep(testDuration)
    close(done)
    wg.Wait()

    // Calculate results
    elapsed := time.Since(startTime)
    averageRPS := float64(totalRequests) / elapsed.Seconds()

    // Print results
    fmt.Printf("\nHTTP/2 Performance Test Results (230s duration):\n")
    fmt.Printf("Target URL: %s\n", url)
    fmt.Printf("Total Requests: %d\n", totalRequests)
    fmt.Printf("Successful Requests: %d\n", successfulRequests)
    fmt.Printf("Failed Requests: %d\n", failedRequests)
    fmt.Printf("Peak RPS: %d\n", peakRPS)
    fmt.Printf("Average RPS: %.2f\n", averageRPS)
    fmt.Printf("Success Rate: %.2f%%\n", float64(successfulRequests)/float64(totalRequests)*100)
}

// fetchProxies downloads proxy lists from multiple sources
func fetchProxies() []string {
    proxySources := []string{
        "https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/http.txt",
        "https://api.proxyscrape.com/v2/?request=getproxies&protocol=http&timeout=10000&country=all",
        "https://raw.githubusercontent.com/clarketm/proxy-list/master/proxy-list-raw.txt",
        "https://raw.githubusercontent.com/ShiftyTR/Proxy-List/master/http.txt",
    }
    var proxies []string

    client := &fasthttp.Client{ReadTimeout: 5 * time.Second}
    req := fasthttp.AcquireRequest()
    resp := fasthttp.AcquireResponse()
    defer fasthttp.ReleaseRequest(req)
    defer fasthttp.ReleaseResponse(resp)

    for _, source := range proxySources {
        req.SetRequestURI(source)
        if err := client.Do(req, resp); err != nil {
            continue
        }
        body := resp.Body()
        lines := strings.Split(string(body), "\n")
        for _, line := range lines {
            line = strings.TrimSpace(line)
            if line != "" && !strings.Contains(line, "#") {
                proxies = append(proxies, line)
            }
        }
    }
    return proxies
}

// Optional: Local HTTP/2 server for testing
func startLocalServer() {
    server := &http.Server{Addr: ":8080"}
    http2.ConfigureServer(server, nil)
    go server.ListenAndServe()
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("OK"))
    })
}
