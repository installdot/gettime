package main

import (
    "fmt"
    "runtime"
    "sync"
    "sync/atomic"
    "time"

    "github.com/valyala/fasthttp"
)

func main() {
    // Configuration
    const testDuration = 230 * time.Second
    url := "http://scam.vn" // Replace with your API URL

    // Optimize runtime
    runtime.GOMAXPROCS(runtime.NumCPU())

    // Statistics
    var totalRequests int64
    var successfulRequests int64
    var failedRequests int64
    var peakRPS int64

    // Create fasthttp client with optimizations
    client := &fasthttp.Client{
        MaxConnsPerHost:     1000000,           // High connection limit
        ReadTimeout:         1 * time.Second, // Fast timeout
        WriteTimeout:        1 * time.Second,
        MaxIdleConnDuration: 0, // Keep connections alive
    }

    // WaitGroup for goroutines
    var wg sync.WaitGroup
    done := make(chan struct{})

    // Track RPS per second
    rpsTicker := time.NewTicker(1 * time.Second)
    defer rpsTicker.Stop()
    var requestsThisSecond int64

    // Start request sender
    startTime := time.Now()
    for i := 0; i < runtime.NumCPU()*100; i++ { // Spawn workers based on CPU cores
        wg.Add(1)
        go func() {
            defer wg.Done()
            for {
                select {
                case <-done:
                    return
                default:
                    atomic.AddInt64(&totalRequests, 1)
                    atomic.AddInt64(&requestsThisSecond, 1)

                    statusCode, _, err := client.Get(nil, url)
                    if err != nil {
                        atomic.AddInt64(&failedRequests, 1)
                        continue
                    }
                    if statusCode >= 200 && statusCode < 300 {
                        atomic.AddInt64(&successfulRequests, 1)
                    } else {
                        atomic.AddInt64(&failedRequests, 1)
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
    fmt.Printf("\nPerformance Test Results (230s duration):\n")
    fmt.Printf("Total Requests: %d\n", totalRequests)
    fmt.Printf("Successful Requests: %d\n", successfulRequests)
    fmt.Printf("Failed Requests: %d\n", failedRequests)
    fmt.Printf("Peak RPS: %d\n", peakRPS)
    fmt.Printf("Average RPS: %.2f\n", averageRPS)
    fmt.Printf("Success Rate: %.2f%%\n", float64(successfulRequests)/float64(totalRequests)*100)
}
