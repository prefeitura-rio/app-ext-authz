package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration
type Config struct {
	// reCAPTCHA Enterprise settings
	RecaptchaProjectID    string
	RecaptchaSiteKey      string
	RecaptchaAction       string
	RecaptchaV3Threshold  float64
	GoogleServiceAccountKey string // Base64 encoded service account JSON

	// Performance settings
	GoogleAPITimeoutSeconds int
	CacheTTLSeconds        int
	CacheFailedTTLSeconds  int
	RedisURL               string

	// Failure handling
	FailureMode                    string
	CircuitBreakerEnabled          bool
	CircuitBreakerFailureThreshold int
	CircuitBreakerRecoveryTime     time.Duration
	HealthCheckIntervalSeconds     int

	// Observability
	OTelEndpoint    string
	OTelServiceName string
	LogLevel        string

	// Server settings
	Port int

	// Development
	MockMode bool
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	config := &Config{
		// Defaults
		RecaptchaProjectID:           "",
		RecaptchaSiteKey:             "",
		RecaptchaAction:              "authz",
		RecaptchaV3Threshold:         0.5,
		GoogleAPITimeoutSeconds:       5,
		CacheTTLSeconds:               30,
		CacheFailedTTLSeconds:         300,
		RedisURL:                      "redis://localhost:6379",
		FailureMode:                   "fail_open",
		CircuitBreakerEnabled:         true,
		CircuitBreakerFailureThreshold: 5,
		CircuitBreakerRecoveryTime:    60 * time.Second,
		HealthCheckIntervalSeconds:    30,
		OTelServiceName:               "recaptcha-authz",
		LogLevel:                      "info",
		Port:                          8080,
	}

	// Required settings
	if projectID := os.Getenv("RECAPTCHA_PROJECT_ID"); projectID != "" {
		config.RecaptchaProjectID = projectID
	} else {
		return nil, fmt.Errorf("RECAPTCHA_PROJECT_ID is required")
	}

	if siteKey := os.Getenv("RECAPTCHA_SITE_KEY"); siteKey != "" {
		config.RecaptchaSiteKey = siteKey
	} else {
		return nil, fmt.Errorf("RECAPTCHA_SITE_KEY is required")
	}

	// Load service account key from base64 encoded environment variable
	if serviceAccountKey := os.Getenv("GOOGLE_SERVICE_ACCOUNT_KEY"); serviceAccountKey != "" {
		config.GoogleServiceAccountKey = serviceAccountKey
	}

	// Optional settings
	if action := os.Getenv("RECAPTCHA_ACTION"); action != "" {
		config.RecaptchaAction = action
	}

	if threshold := os.Getenv("RECAPTCHA_V3_THRESHOLD"); threshold != "" {
		if t, err := strconv.ParseFloat(threshold, 64); err == nil && t >= 0.0 && t <= 1.0 {
			config.RecaptchaV3Threshold = t
		} else {
			return nil, fmt.Errorf("RECAPTCHA_V3_THRESHOLD must be between 0.0 and 1.0")
		}
	}

	if timeout := os.Getenv("GOOGLE_API_TIMEOUT_SECONDS"); timeout != "" {
		if t, err := strconv.Atoi(timeout); err == nil && t > 0 {
			config.GoogleAPITimeoutSeconds = t
		} else {
			return nil, fmt.Errorf("GOOGLE_API_TIMEOUT_SECONDS must be a positive integer")
		}
	}

	if ttl := os.Getenv("CACHE_TTL_SECONDS"); ttl != "" {
		if t, err := strconv.Atoi(ttl); err == nil && t > 0 {
			config.CacheTTLSeconds = t
		} else {
			return nil, fmt.Errorf("CACHE_TTL_SECONDS must be a positive integer")
		}
	}

	if failedTTL := os.Getenv("CACHE_FAILED_TTL_SECONDS"); failedTTL != "" {
		if t, err := strconv.Atoi(failedTTL); err == nil && t > 0 {
			config.CacheFailedTTLSeconds = t
		} else {
			return nil, fmt.Errorf("CACHE_FAILED_TTL_SECONDS must be a positive integer")
		}
	}

	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		config.RedisURL = redisURL
	}

	if mode := os.Getenv("FAILURE_MODE"); mode != "" {
		if mode == "fail_open" || mode == "fail_closed" {
			config.FailureMode = mode
		} else {
			return nil, fmt.Errorf("FAILURE_MODE must be 'fail_open' or 'fail_closed'")
		}
	}

	if enabled := os.Getenv("CIRCUIT_BREAKER_ENABLED"); enabled != "" {
		config.CircuitBreakerEnabled = strings.ToLower(enabled) == "true"
	}

	if threshold := os.Getenv("CIRCUIT_BREAKER_FAILURE_THRESHOLD"); threshold != "" {
		if t, err := strconv.Atoi(threshold); err == nil && t > 0 {
			config.CircuitBreakerFailureThreshold = t
		} else {
			return nil, fmt.Errorf("CIRCUIT_BREAKER_FAILURE_THRESHOLD must be a positive integer")
		}
	}

	if recoveryTime := os.Getenv("CIRCUIT_BREAKER_RECOVERY_TIME_SECONDS"); recoveryTime != "" {
		if t, err := strconv.Atoi(recoveryTime); err == nil && t > 0 {
			config.CircuitBreakerRecoveryTime = time.Duration(t) * time.Second
		} else {
			return nil, fmt.Errorf("CIRCUIT_BREAKER_RECOVERY_TIME_SECONDS must be a positive integer")
		}
	}

	if interval := os.Getenv("HEALTH_CHECK_INTERVAL_SECONDS"); interval != "" {
		if t, err := strconv.Atoi(interval); err == nil && t > 0 {
			config.HealthCheckIntervalSeconds = t
		} else {
			return nil, fmt.Errorf("HEALTH_CHECK_INTERVAL_SECONDS must be a positive integer")
		}
	}

	// Observability
	if endpoint := os.Getenv("OTEL_ENDPOINT"); endpoint != "" {
		config.OTelEndpoint = endpoint
	}

	if serviceName := os.Getenv("OTEL_SERVICE_NAME"); serviceName != "" {
		config.OTelServiceName = serviceName
	}

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		config.LogLevel = strings.ToLower(logLevel)
	}

	// Server settings
	if port := os.Getenv("PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil && p > 0 && p < 65536 {
			config.Port = p
		} else {
			return nil, fmt.Errorf("PORT must be a valid port number (1-65535)")
		}
	}

	// Development mode
	config.MockMode = strings.ToLower(os.Getenv("MOCK_MODE")) == "true"

	return config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.RecaptchaProjectID == "" {
		return fmt.Errorf("recaptcha project ID is required")
	}

	if c.RecaptchaSiteKey == "" {
		return fmt.Errorf("recaptcha site key is required")
	}

	if c.RecaptchaV3Threshold < 0.0 || c.RecaptchaV3Threshold > 1.0 {
		return fmt.Errorf("recaptcha v3 threshold must be between 0.0 and 1.0")
	}

	if c.GoogleAPITimeoutSeconds <= 0 {
		return fmt.Errorf("google API timeout must be positive")
	}

	if c.CacheTTLSeconds <= 0 {
		return fmt.Errorf("cache TTL must be positive")
	}

	if c.CacheFailedTTLSeconds <= 0 {
		return fmt.Errorf("failed cache TTL must be positive")
	}

	if c.RedisURL == "" {
		return fmt.Errorf("redis URL is required")
	}

	if c.FailureMode != "fail_open" && c.FailureMode != "fail_closed" {
		return fmt.Errorf("failure mode must be 'fail_open' or 'fail_closed'")
	}

	if c.CircuitBreakerFailureThreshold <= 0 {
		return fmt.Errorf("circuit breaker failure threshold must be positive")
	}

	if c.CircuitBreakerRecoveryTime <= 0 {
		return fmt.Errorf("circuit breaker recovery time must be positive")
	}

	if c.HealthCheckIntervalSeconds <= 0 {
		return fmt.Errorf("health check interval must be positive")
	}

	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	return nil
}

// String returns a string representation of the config (without sensitive data)
func (c *Config) String() string {
	return fmt.Sprintf(
		"Config{ProjectID: %s, SiteKey: %s, Action: %s, V3Threshold: %.2f, Timeout: %ds, CacheTTL: %ds, RedisURL: %s, FailureMode: %s, CircuitBreaker: %t, Port: %d, MockMode: %t}",
		c.RecaptchaProjectID,
		c.RecaptchaSiteKey,
		c.RecaptchaAction,
		c.RecaptchaV3Threshold,
		c.GoogleAPITimeoutSeconds,
		c.CacheTTLSeconds,
		c.RedisURL,
		c.FailureMode,
		c.CircuitBreakerEnabled,
		c.Port,
		c.MockMode,
	)
} 