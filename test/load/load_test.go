//go:build load

package load

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/prefeitura-rio/app-ext-authz/internal/config"
	"github.com/prefeitura-rio/app-ext-authz/internal/service"
)

// LoadTestConfig holds load test configuration
type LoadTestConfig struct {
	ConcurrentUsers int
	RequestsPerUser int
	TestDuration    time.Duration
	RampUpTime      time.Duration
}

// LoadTestResult holds load test results
type LoadTestResult struct {
	TotalRequests     int
	SuccessfulRequests int
	FailedRequests    int
	AverageResponseTime time.Duration
	MinResponseTime    time.Duration
	MaxResponseTime    time.Duration
	RequestsPerSecond  float64
	ErrorRate          float64
}

func TestLoad_ConcurrentUsers(t *testing.T) {
	configs := []LoadTestConfig{
		{
			ConcurrentUsers: 10,
			RequestsPerUser: 100,
			TestDuration:    30 * time.Second,
			RampUpTime:      5 * time.Second,
		},
		{
			ConcurrentUsers: 50,
			RequestsPerUser: 50,
			TestDuration:    30 * time.Second,
			RampUpTime:      10 * time.Second,
		},
		{
			ConcurrentUsers: 100,
			RequestsPerUser: 25,
			TestDuration:    30 * time.Second,
			RampUpTime:      15 * time.Second,
		},
	}

	for _, cfg := range configs {
		t.Run(fmt.Sprintf("ConcurrentUsers_%d", cfg.ConcurrentUsers), func(t *testing.T) {
			result := runLoadTest(t, cfg)
			t.Logf("Load test results: %+v", result)

			// Assertions
			if result.ErrorRate > 0.05 { // 5% error rate threshold
				t.Errorf("Error rate too high: %.2f%%", result.ErrorRate*100)
			}

			if result.AverageResponseTime > 100*time.Millisecond {
				t.Errorf("Average response time too high: %v", result.AverageResponseTime)
			}

			if result.RequestsPerSecond < 100 {
				t.Errorf("Throughput too low: %.2f req/s", result.RequestsPerSecond)
			}
		})
	}
}

func TestLoad_CachePerformance(t *testing.T) {
	cfg := LoadTestConfig{
		ConcurrentUsers: 20,
		RequestsPerUser: 50,
		TestDuration:    20 * time.Second,
		RampUpTime:      5 * time.Second,
	}

	// Test with cache hits (same tokens)
	t.Run("CacheHits", func(t *testing.T) {
		result := runLoadTestWithTokens(t, cfg, []string{"valid_token", "valid_token", "valid_token"})
		t.Logf("Cache hits test results: %+v", result)

		// Cache hits should be faster
		if result.AverageResponseTime > 50*time.Millisecond {
			t.Errorf("Cache hit response time too high: %v", result.AverageResponseTime)
		}
	})

	// Test with cache misses (unique tokens)
	t.Run("CacheMisses", func(t *testing.T) {
		tokens := generateUniqueTokens(100)
		result := runLoadTestWithTokens(t, cfg, tokens)
		t.Logf("Cache misses test results: %+v", result)

		// Cache misses should be slower but still reasonable
		if result.AverageResponseTime > 200*time.Millisecond {
			t.Errorf("Cache miss response time too high: %v", result.AverageResponseTime)
		}
	})
}

func TestLoad_CircuitBreaker(t *testing.T) {
	cfg := LoadTestConfig{
		ConcurrentUsers: 10,
		RequestsPerUser: 20,
		TestDuration:    15 * time.Second,
		RampUpTime:      5 * time.Second,
	}

	// Test with timeout tokens to trigger circuit breaker
	t.Run("CircuitBreakerTrigger", func(t *testing.T) {
		result := runLoadTestWithTokens(t, cfg, []string{"timeout_token", "timeout_token", "timeout_token"})
		t.Logf("Circuit breaker test results: %+v", result)

		// Should have some failures but not complete failure
		if result.ErrorRate > 0.8 {
			t.Errorf("Error rate too high with circuit breaker: %.2f%%", result.ErrorRate*100)
		}
	})
}

func TestLoad_MixedWorkload(t *testing.T) {
	cfg := LoadTestConfig{
		ConcurrentUsers: 30,
		RequestsPerUser: 40,
		TestDuration:    25 * time.Second,
		RampUpTime:      10 * time.Second,
	}

	// Mixed workload with different token types
	tokens := []string{
		"valid_token",      // 40% - valid
		"invalid_token",    // 30% - invalid
		"low_score_token",  // 20% - low score
		"timeout_token",    // 10% - timeout
	}

	t.Run("MixedWorkload", func(t *testing.T) {
		result := runLoadTestWithTokens(t, cfg, tokens)
		t.Logf("Mixed workload test results: %+v", result)

		// Mixed workload should have moderate error rate
		if result.ErrorRate > 0.3 {
			t.Errorf("Error rate too high for mixed workload: %.2f%%", result.ErrorRate*100)
		}

		// Should maintain reasonable throughput
		if result.RequestsPerSecond < 50 {
			t.Errorf("Throughput too low for mixed workload: %.2f req/s", result.RequestsPerSecond)
		}
	})
}

func TestLoad_StressTest(t *testing.T) {
	cfg := LoadTestConfig{
		ConcurrentUsers: 200,
		RequestsPerUser: 100,
		TestDuration:    60 * time.Second,
		RampUpTime:      30 * time.Second,
	}

	t.Run("StressTest", func(t *testing.T) {
		result := runLoadTest(t, cfg)
		t.Logf("Stress test results: %+v", result)

		// Stress test should maintain stability
		if result.ErrorRate > 0.1 {
			t.Errorf("Error rate too high in stress test: %.2f%%", result.ErrorRate*100)
		}

		// Should handle high load
		if result.RequestsPerSecond < 200 {
			t.Errorf("Throughput too low in stress test: %.2f req/s", result.RequestsPerSecond)
		}
	})
}

// runLoadTest runs a load test with the given configuration
func runLoadTest(t *testing.T, cfg LoadTestConfig) LoadTestResult {
	// Create service configuration
	svcConfig := &config.Config{
		RecaptchaProjectID:           "test-project",
		RecaptchaSiteKey:             "test_site_key",
		RecaptchaAction:              "authz",
		RecaptchaV3Threshold:         0.5,
		GoogleAPITimeoutSeconds:      5,
		CacheTTLSeconds:              30,
		CacheFailedTTLSeconds:        300,
		RedisURL:                     "redis://localhost:6379",
		FailureMode:                  "fail_open",
		CircuitBreakerEnabled:        true,
		CircuitBreakerFailureThreshold: 5,
		CircuitBreakerRecoveryTime:   60 * time.Second,
		HealthCheckIntervalSeconds:   30,
		OTelServiceName:              "load-test",
		LogLevel:                     "error", // Reduce logging noise
		Port:                         8080,
		MockMode:                     true,
	}

	// Create service
	svc, err := service.NewService(svcConfig)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer svc.Shutdown(context.Background())

	// Test tokens
	tokens := []string{"valid_token", "invalid_token", "low_score_token"}

	// Run load test
	return runLoadTestWithService(t, cfg, svc, tokens)
}

// runLoadTestWithTokens runs a load test with specific tokens
func runLoadTestWithTokens(t *testing.T, cfg LoadTestConfig, tokens []string) LoadTestResult {
	// Create service configuration
	svcConfig := &config.Config{
		RecaptchaProjectID:           "test-project",
		RecaptchaSiteKey:             "test_site_key",
		RecaptchaAction:              "authz",
		RecaptchaV3Threshold:         0.5,
		GoogleAPITimeoutSeconds:      5,
		CacheTTLSeconds:              30,
		CacheFailedTTLSeconds:        300,
		RedisURL:                     "redis://localhost:6379",
		FailureMode:                  "fail_open",
		CircuitBreakerEnabled:        true,
		CircuitBreakerFailureThreshold: 5,
		CircuitBreakerRecoveryTime:   60 * time.Second,
		HealthCheckIntervalSeconds:   30,
		OTelServiceName:              "load-test",
		LogLevel:                     "error",
		Port:                         8080,
		MockMode:                     true,
	}

	// Create service
	svc, err := service.NewService(svcConfig)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer svc.Shutdown(context.Background())

	// Run load test
	return runLoadTestWithService(t, cfg, svc, tokens)
}

// runLoadTestWithService runs a load test with an existing service
func runLoadTestWithService(t *testing.T, cfg LoadTestConfig, svc *service.Service, tokens []string) LoadTestResult {
	var (
		wg                sync.WaitGroup
		mu                sync.Mutex
		totalRequests     int
		successfulRequests int
		failedRequests    int
		responseTimes     []time.Duration
		startTime         = time.Now()
	)

	// Start concurrent users
	for i := 0; i < cfg.ConcurrentUsers; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()

			// Ramp up delay
			rampUpDelay := time.Duration(userID) * cfg.RampUpTime / time.Duration(cfg.ConcurrentUsers)
			time.Sleep(rampUpDelay)

			// Make requests
			for j := 0; j < cfg.RequestsPerUser; j++ {
				// Select token (round-robin)
				token := tokens[j%len(tokens)]

				req := &service.AuthorizationRequest{
					Token: token,
				}

				requestStart := time.Now()
				response, err := svc.Authorize(context.Background(), req)
				responseTime := time.Since(requestStart)

				mu.Lock()
				totalRequests++
				if err == nil && response != nil {
					successfulRequests++
				} else {
					failedRequests++
				}
				responseTimes = append(responseTimes, responseTime)
				mu.Unlock()

				// Small delay between requests
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	// Wait for all users to complete or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All users completed
	case <-time.After(cfg.TestDuration):
		// Timeout reached
		t.Logf("Load test timed out after %v", cfg.TestDuration)
	}

	// Calculate results
	duration := time.Since(startTime)
	
	mu.Lock()
	defer mu.Unlock()

	// Calculate response time statistics
	var totalResponseTime time.Duration
	minResponseTime := responseTimes[0]
	maxResponseTime := responseTimes[0]

	for _, rt := range responseTimes {
		totalResponseTime += rt
		if rt < minResponseTime {
			minResponseTime = rt
		}
		if rt > maxResponseTime {
			maxResponseTime = rt
		}
	}

	avgResponseTime := totalResponseTime / time.Duration(len(responseTimes))
	requestsPerSecond := float64(totalRequests) / duration.Seconds()
	errorRate := float64(failedRequests) / float64(totalRequests)

	return LoadTestResult{
		TotalRequests:      totalRequests,
		SuccessfulRequests: successfulRequests,
		FailedRequests:     failedRequests,
		AverageResponseTime: avgResponseTime,
		MinResponseTime:     minResponseTime,
		MaxResponseTime:     maxResponseTime,
		RequestsPerSecond:   requestsPerSecond,
		ErrorRate:           errorRate,
	}
}

// generateUniqueTokens generates a slice of unique tokens for testing
func generateUniqueTokens(count int) []string {
	tokens := make([]string, count)
	for i := 0; i < count; i++ {
		tokens[i] = fmt.Sprintf("unique_token_%d", i)
	}
	return tokens
}

// Benchmark tests for performance measurement
func BenchmarkService_Authorize(b *testing.B) {
	// Create service configuration
	cfg := &config.Config{
		RecaptchaProjectID:           "test-project",
		RecaptchaSiteKey:             "test_site_key",
		RecaptchaAction:              "authz",
		RecaptchaV3Threshold:         0.5,
		GoogleAPITimeoutSeconds:      5,
		CacheTTLSeconds:              30,
		CacheFailedTTLSeconds:        300,
		RedisURL:                     "redis://localhost:6379",
		FailureMode:                  "fail_open",
		CircuitBreakerEnabled:        true,
		CircuitBreakerFailureThreshold: 5,
		CircuitBreakerRecoveryTime:   60 * time.Second,
		HealthCheckIntervalSeconds:   30,
		OTelServiceName:              "benchmark",
		LogLevel:                     "error",
		Port:                         8080,
		MockMode:                     true,
	}

	// Create service
	svc, err := service.NewService(cfg)
	if err != nil {
		b.Fatalf("Failed to create service: %v", err)
	}
	defer svc.Shutdown(context.Background())

	// Test tokens
	tokens := []string{"valid_token", "invalid_token", "low_score_token"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			token := tokens[i%len(tokens)]
			req := &service.AuthorizationRequest{
				Token: token,
			}

			_, err := svc.Authorize(context.Background(), req)
			if err != nil {
				b.Errorf("Authorization failed: %v", err)
			}
			i++
		}
	})
} 