package main

import (
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// PerformanceAnalyzer conducts comprehensive performance testing
type PerformanceAnalyzer struct {
	BaseURL    string
	Client     *http.Client
	Results    []PerformanceResult
}

type PerformanceResult struct {
	Test           string
	Duration       time.Duration
	RequestsPerSec float64
	SuccessRate    float64
	AvgLatency     time.Duration
	MaxLatency     time.Duration
	MinLatency     time.Duration
	Errors         int
	Status         string
}

// NewPerformanceAnalyzer creates a new performance analyzer
func NewPerformanceAnalyzer(baseURL string) *PerformanceAnalyzer {
	return &PerformanceAnalyzer{
		BaseURL: baseURL,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
		Results: make([]PerformanceResult, 0),
	}
}

// RunAllTests executes comprehensive performance testing
func (pa *PerformanceAnalyzer) RunAllTests() {
	fmt.Println("⚡ APEX.BUILD Performance Analysis Starting...")

	pa.TestSingleRequest()
	pa.TestConcurrentRequests(10)
	pa.TestConcurrentRequests(50)
	pa.TestConcurrentRequests(100)
	pa.TestLoadSustainability()
	pa.TestMemoryUsage()
	pa.TestResponseTimes()

	pa.GenerateReport()
}

// TestSingleRequest tests baseline single request performance
func (pa *PerformanceAnalyzer) TestSingleRequest() {
	start := time.Now()

	resp, err := pa.Client.Get(pa.BaseURL + "/health")
	duration := time.Since(start)

	if err != nil {
		pa.Results = append(pa.Results, PerformanceResult{
			Test:     "Single Request",
			Duration: duration,
			Errors:   1,
			Status:   "FAIL",
		})
		return
	}
	defer resp.Body.Close()

	pa.Results = append(pa.Results, PerformanceResult{
		Test:        "Single Request",
		Duration:    duration,
		AvgLatency:  duration,
		SuccessRate: 100.0,
		Status:      "PASS",
	})
}

// TestConcurrentRequests tests system under concurrent load
func (pa *PerformanceAnalyzer) TestConcurrentRequests(concurrency int) {
	fmt.Printf("Testing with %d concurrent requests...\n", concurrency)

	var wg sync.WaitGroup
	results := make(chan time.Duration, concurrency)
	errors := make(chan error, concurrency)

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			reqStart := time.Now()
			resp, err := pa.Client.Get(pa.BaseURL + "/health")
			reqDuration := time.Since(reqStart)

			if err != nil {
				errors <- err
				return
			}
			defer resp.Body.Close()

			// Read response to ensure complete processing
			io.ReadAll(resp.Body)
			results <- reqDuration
		}()
	}

	wg.Wait()
	close(results)
	close(errors)

	totalDuration := time.Since(start)

	// Calculate statistics
	var latencies []time.Duration
	errorCount := 0

	for latency := range results {
		latencies = append(latencies, latency)
	}

	for range errors {
		errorCount++
	}

	if len(latencies) == 0 {
		pa.Results = append(pa.Results, PerformanceResult{
			Test:   fmt.Sprintf("Concurrent Requests (%d)", concurrency),
			Errors: errorCount,
			Status: "FAIL",
		})
		return
	}

	// Calculate averages
	var totalLatency time.Duration
	minLatency := latencies[0]
	maxLatency := latencies[0]

	for _, latency := range latencies {
		totalLatency += latency
		if latency < minLatency {
			minLatency = latency
		}
		if latency > maxLatency {
			maxLatency = latency
		}
	}

	avgLatency := totalLatency / time.Duration(len(latencies))
	successRate := (float64(len(latencies)) / float64(concurrency)) * 100
	requestsPerSec := float64(len(latencies)) / totalDuration.Seconds()

	status := "PASS"
	if successRate < 95.0 || avgLatency > 1*time.Second {
		status = "FAIL"
	}

	pa.Results = append(pa.Results, PerformanceResult{
		Test:           fmt.Sprintf("Concurrent Requests (%d)", concurrency),
		Duration:       totalDuration,
		RequestsPerSec: requestsPerSec,
		SuccessRate:    successRate,
		AvgLatency:     avgLatency,
		MaxLatency:     maxLatency,
		MinLatency:     minLatency,
		Errors:         errorCount,
		Status:         status,
	})
}

// TestLoadSustainability tests sustained load over time
func (pa *PerformanceAnalyzer) TestLoadSustainability() {
	fmt.Println("Testing load sustainability...")

	concurrency := 20
	duration := 30 * time.Second

	var totalRequests int
	var totalErrors int
	var totalLatency time.Duration

	start := time.Now()
	end := start.Add(duration)

	var wg sync.WaitGroup
	results := make(chan time.Duration, 1000)
	errors := make(chan error, 1000)

	// Launch workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for time.Now().Before(end) {
				reqStart := time.Now()
				resp, err := pa.Client.Get(pa.BaseURL + "/health")
				reqDuration := time.Since(reqStart)

				if err != nil {
					errors <- err
					continue
				}

				resp.Body.Close()
				results <- reqDuration

				// Small delay to prevent overwhelming
				time.Sleep(100 * time.Millisecond)
			}
		}()
	}

	wg.Wait()
	close(results)
	close(errors)

	totalDuration := time.Since(start)

	// Collect results
	for latency := range results {
		totalRequests++
		totalLatency += latency
	}

	for range errors {
		totalErrors++
	}

	if totalRequests == 0 {
		pa.Results = append(pa.Results, PerformanceResult{
			Test:   "Load Sustainability",
			Errors: totalErrors,
			Status: "FAIL",
		})
		return
	}

	avgLatency := totalLatency / time.Duration(totalRequests)
	successRate := (float64(totalRequests) / float64(totalRequests+totalErrors)) * 100
	requestsPerSec := float64(totalRequests) / totalDuration.Seconds()

	status := "PASS"
	if successRate < 95.0 || avgLatency > 2*time.Second {
		status = "FAIL"
	}

	pa.Results = append(pa.Results, PerformanceResult{
		Test:           "Load Sustainability",
		Duration:       totalDuration,
		RequestsPerSec: requestsPerSec,
		SuccessRate:    successRate,
		AvgLatency:     avgLatency,
		Errors:         totalErrors,
		Status:         status,
	})
}

// TestMemoryUsage monitors memory usage during testing
func (pa *PerformanceAnalyzer) TestMemoryUsage() {
	fmt.Println("Testing memory usage...")

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	// Simulate load
	for i := 0; i < 100; i++ {
		resp, err := pa.Client.Get(pa.BaseURL + "/health")
		if err == nil {
			resp.Body.Close()
		}
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	memoryIncreaseKB := float64(m2.Alloc-m1.Alloc) / 1024

	status := "PASS"
	if memoryIncreaseKB > 10*1024 { // 10MB increase is concerning
		status = "REVIEW"
	}

	pa.Results = append(pa.Results, PerformanceResult{
		Test:   fmt.Sprintf("Memory Usage (increase: %.2f KB)", memoryIncreaseKB),
		Status: status,
	})
}

// TestResponseTimes tests response times for different endpoints
func (pa *PerformanceAnalyzer) TestResponseTimes() {
	endpoints := []string{
		"/health",
		"/api/v1/auth/login",
		"/api/v1/ai/generate",
	}

	for _, endpoint := range endpoints {
		start := time.Now()
		resp, err := pa.Client.Get(pa.BaseURL + endpoint)
		latency := time.Since(start)

		status := "PASS"
		if err != nil {
			status = "ERROR"
		} else {
			resp.Body.Close()
			if latency > 5*time.Second {
				status = "SLOW"
			}
		}

		pa.Results = append(pa.Results, PerformanceResult{
			Test:       fmt.Sprintf("Response Time %s", endpoint),
			AvgLatency: latency,
			Status:     status,
		})
	}
}

// GenerateReport generates the performance analysis report
func (pa *PerformanceAnalyzer) GenerateReport() {
	fmt.Println("\n⚡ APEX.BUILD PERFORMANCE ANALYSIS REPORT")
	fmt.Println("=========================================")

	passed := 0
	failed := 0

	for _, result := range pa.Results {
		switch result.Status {
		case "PASS":
			passed++
		case "FAIL":
			failed++
		}

		fmt.Printf("\n[%s] %s\n", result.Status, result.Test)

		if result.RequestsPerSec > 0 {
			fmt.Printf("  Requests/sec: %.2f\n", result.RequestsPerSec)
		}
		if result.SuccessRate > 0 {
			fmt.Printf("  Success Rate: %.2f%%\n", result.SuccessRate)
		}
		if result.AvgLatency > 0 {
			fmt.Printf("  Avg Latency: %v\n", result.AvgLatency)
		}
		if result.MaxLatency > 0 {
			fmt.Printf("  Max Latency: %v\n", result.MaxLatency)
		}
		if result.MinLatency > 0 {
			fmt.Printf("  Min Latency: %v\n", result.MinLatency)
		}
		if result.Errors > 0 {
			fmt.Printf("  Errors: %d\n", result.Errors)
		}
		if result.Duration > 0 {
			fmt.Printf("  Duration: %v\n", result.Duration)
		}
	}

	fmt.Printf("\n📊 PERFORMANCE SUMMARY\n")
	fmt.Printf("======================\n")
	fmt.Printf("Tests Passed: %d\n", passed)
	fmt.Printf("Tests Failed: %d\n", failed)

	if failed == 0 {
		fmt.Printf("\n✅ PERFORMANCE STATUS: EXCELLENT\n")
		fmt.Printf("All performance tests passed successfully.\n")
	} else if failed < 3 {
		fmt.Printf("\n⚠️  PERFORMANCE STATUS: GOOD\n")
		fmt.Printf("Minor performance issues detected.\n")
	} else {
		fmt.Printf("\n🚨 PERFORMANCE STATUS: NEEDS OPTIMIZATION\n")
		fmt.Printf("Multiple performance issues detected.\n")
	}

	// Recommendations
	fmt.Printf("\n🎯 RECOMMENDATIONS\n")
	fmt.Printf("==================\n")

	highLatency := false
	lowThroughput := false

	for _, result := range pa.Results {
		if result.AvgLatency > 1*time.Second {
			highLatency = true
		}
		if result.RequestsPerSec > 0 && result.RequestsPerSec < 100 {
			lowThroughput = true
		}
	}

	if highLatency {
		fmt.Printf("• Consider implementing response caching\n")
		fmt.Printf("• Optimize database queries\n")
		fmt.Printf("• Review AI service response times\n")
	}

	if lowThroughput {
		fmt.Printf("• Consider implementing connection pooling\n")
		fmt.Printf("• Optimize middleware performance\n")
		fmt.Printf("• Review server configuration\n")
	}

	if !highLatency && !lowThroughput {
		fmt.Printf("• System performance is within acceptable ranges\n")
		fmt.Printf("• Consider adding performance monitoring in production\n")
		fmt.Printf("• Set up automated performance regression testing\n")
	}
}

func main() {
	analyzer := NewPerformanceAnalyzer("http://localhost:8080")
	analyzer.RunAllTests()
}