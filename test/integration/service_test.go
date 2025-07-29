//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/prefeitura-rio/app-ext-authz/internal/config"
	"github.com/prefeitura-rio/app-ext-authz/internal/service"
)

func TestService_Authorize_Integration(t *testing.T) {
	// Setup test configuration
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
		OTelServiceName:              "test-service",
		LogLevel:                     "debug",
		Port:                         8080,
		MockMode:                     true,
	}

	// Create service
	svc, err := service.NewService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer svc.Shutdown(context.Background())

	tests := []struct {
		name           string
		token          string
		expectedAllowed bool
		expectedStatus  string
		expectedCache   string
	}{
		{
			name:           "valid token",
			token:          "valid_token",
			expectedAllowed: true,
			expectedStatus:  "valid",
			expectedCache:   "miss",
		},
		{
			name:           "invalid token",
			token:          "invalid_token",
			expectedAllowed: false,
			expectedStatus:  "invalid",
			expectedCache:   "miss",
		},
		{
			name:           "low score token",
			token:          "low_score_token",
			expectedAllowed: false,
			expectedStatus:  "score-below-threshold",
			expectedCache:   "miss",
		},
		{
			name:           "empty token",
			token:          "",
			expectedAllowed: false,
			expectedStatus:  "missing-input-response",
			expectedCache:   "miss",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &service.AuthorizationRequest{
				Token: tt.token,
			}

			response, err := svc.Authorize(context.Background(), req)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if response.Allowed != tt.expectedAllowed {
				t.Errorf("Expected allowed=%v, got %v", tt.expectedAllowed, response.Allowed)
			}

			if response.Status != tt.expectedStatus {
				t.Errorf("Expected status=%v, got %v", tt.expectedStatus, response.Status)
			}

			if response.Cache != tt.expectedCache {
				t.Errorf("Expected cache=%v, got %v", tt.expectedCache, response.Cache)
			}
		})
	}
}

func TestService_Authorize_Cache_Integration(t *testing.T) {
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
		OTelServiceName:              "test-service",
		LogLevel:                     "debug",
		Port:                         8080,
		MockMode:                     true,
	}

	svc, err := service.NewService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer svc.Shutdown(context.Background())

	// First request - should be cache miss
	req1 := &service.AuthorizationRequest{
		Token: "valid_token",
	}

	response1, err := svc.Authorize(context.Background(), req1)
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}

	if response1.Cache != "miss" {
		t.Errorf("Expected first request to be cache miss, got %v", response1.Cache)
	}

	// Second request with same token - should be cache hit
	req2 := &service.AuthorizationRequest{
		Token: "valid_token",
	}

	response2, err := svc.Authorize(context.Background(), req2)
	if err != nil {
		t.Fatalf("Second request failed: %v", err)
	}

	if response2.Cache != "hit" {
		t.Errorf("Expected second request to be cache hit, got %v", response2.Cache)
	}

	// Results should be the same
	if response1.Allowed != response2.Allowed {
		t.Errorf("Cache hit result differs from cache miss result")
	}

	if response1.Status != response2.Status {
		t.Errorf("Cache hit status differs from cache miss status")
	}
}

func TestService_Authorize_CircuitBreaker_Integration(t *testing.T) {
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
		CircuitBreakerFailureThreshold: 2, // Low threshold for testing
		CircuitBreakerRecoveryTime:   1 * time.Second, // Short recovery for testing
		HealthCheckIntervalSeconds:   30,
		OTelServiceName:              "test-service",
		LogLevel:                     "debug",
		Port:                         8080,
		MockMode:                     true,
	}

	svc, err := service.NewService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer svc.Shutdown(context.Background())

	// Make requests that will trigger circuit breaker
	for i := 0; i < 3; i++ {
		req := &service.AuthorizationRequest{
			Token: "timeout_token", // This will cause timeout
		}

		response, err := svc.Authorize(context.Background(), req)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}

		// First two should be timeout, third should be degraded due to circuit breaker
		if i < 2 {
			if response.Status != "timeout" {
				t.Errorf("Request %d: Expected status 'timeout', got '%v'", i+1, response.Status)
			}
		} else {
			if response.Status != "degraded" {
				t.Errorf("Request %d: Expected status 'degraded' due to circuit breaker, got '%v'", i+1, response.Status)
			}
		}
	}

	// Wait for circuit breaker to recover
	time.Sleep(2 * time.Second)

	// Try a valid request - should work again
	req := &service.AuthorizationRequest{
		Token: "valid_token",
	}

	response, err := svc.Authorize(context.Background(), req)
	if err != nil {
		t.Fatalf("Recovery request failed: %v", err)
	}

	if response.Status != "valid" {
		t.Errorf("Expected recovery request to be valid, got '%v'", response.Status)
	}
}

func TestService_Authorize_FailClosed_Integration(t *testing.T) {
	cfg := &config.Config{
		RecaptchaProjectID:           "test-project",
		RecaptchaSiteKey:             "test_site_key",
		RecaptchaAction:              "authz",
		RecaptchaV3Threshold:         0.5,
		GoogleAPITimeoutSeconds:      5,
		CacheTTLSeconds:              30,
		CacheFailedTTLSeconds:        300,
		RedisURL:                     "redis://localhost:6379",
		FailureMode:                  "fail_closed", // Different failure mode
		CircuitBreakerEnabled:        true,
		CircuitBreakerFailureThreshold: 2,
		CircuitBreakerRecoveryTime:   1 * time.Second,
		HealthCheckIntervalSeconds:   30,
		OTelServiceName:              "test-service",
		LogLevel:                     "debug",
		Port:                         8080,
		MockMode:                     true,
	}

	svc, err := service.NewService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer svc.Shutdown(context.Background())

	// Trigger circuit breaker
	for i := 0; i < 3; i++ {
		req := &service.AuthorizationRequest{
			Token: "timeout_token",
		}

		response, err := svc.Authorize(context.Background(), req)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}

		if i < 2 {
			if response.Status != "timeout" {
				t.Errorf("Request %d: Expected status 'timeout', got '%v'", i+1, response.Status)
			}
		} else {
			if response.Status != "circuit_breaker_open" {
				t.Errorf("Request %d: Expected status 'circuit_breaker_open' due to fail_closed mode, got '%v'", i+1, response.Status)
			}
			if response.Allowed {
				t.Errorf("Request %d: Expected request to be denied in fail_closed mode", i+1)
			}
		}
	}
}

func TestService_GetHealth_Integration(t *testing.T) {
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
		OTelServiceName:              "test-service",
		LogLevel:                     "debug",
		Port:                         8080,
		MockMode:                     true,
	}

	svc, err := service.NewService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer svc.Shutdown(context.Background())

	health := svc.GetHealth()

	// Check required fields
	if health["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got '%v'", health["status"])
	}

	if health["timestamp"] == "" {
		t.Error("Expected timestamp to be present")
	}

	// Check circuit breaker info
	cb, ok := health["circuit_breaker"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected circuit_breaker field to be present")
	}

	if cb["state"] == "" {
		t.Error("Expected circuit breaker state to be present")
	}

	// Check cache info
	cache, ok := health["cache"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected cache field to be present")
	}

	if cache["hits"] == nil {
		t.Error("Expected cache hits to be present")
	}

	if cache["misses"] == nil {
		t.Error("Expected cache misses to be present")
	}

	// Check config info
	config, ok := health["config"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected config field to be present")
	}

	if config["recaptcha_version"] != float64(3) {
		t.Errorf("Expected recaptcha_version to be 3, got %v", config["recaptcha_version"])
	}

	if config["failure_mode"] != "fail_open" {
		t.Errorf("Expected failure_mode to be 'fail_open', got '%v'", config["failure_mode"])
	}
}

func TestService_GetMetrics_Integration(t *testing.T) {
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
		OTelServiceName:              "test-service",
		LogLevel:                     "debug",
		Port:                         8080,
		MockMode:                     true,
	}

	svc, err := service.NewService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer svc.Shutdown(context.Background())

	// Make some requests to generate metrics
	for i := 0; i < 5; i++ {
		req := &service.AuthorizationRequest{
			Token: "valid_token",
		}
		_, err := svc.Authorize(context.Background(), req)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}
	}

	metrics := svc.GetMetrics()

	// Check circuit breaker metrics
	cb, ok := metrics["circuit_breaker"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected circuit_breaker field to be present")
	}

	if cb["total_requests"] == nil {
		t.Error("Expected total_requests to be present")
	}

	// Check cache metrics
	cache, ok := metrics["cache"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected cache field to be present")
	}

	if cache["hits"] == nil {
		t.Error("Expected cache hits to be present")
	}

	if cache["misses"] == nil {
		t.Error("Expected cache misses to be present")
	}
} 