package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/prefeitura-rio/app-ext-authz/internal/cache"
	"github.com/prefeitura-rio/app-ext-authz/internal/circuitbreaker"
	"github.com/prefeitura-rio/app-ext-authz/internal/config"
	"github.com/prefeitura-rio/app-ext-authz/internal/observability"
	"github.com/prefeitura-rio/app-ext-authz/internal/recaptcha"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Service handles authorization requests
type Service struct {
	config         *config.Config
	recaptchaClient recaptcha.Client
	cache          cache.Cache
	circuitBreaker *circuitbreaker.Breaker
	telemetry      *observability.Telemetry
	metrics        *observability.Metrics
}

// AuthorizationRequest represents an authorization request
type AuthorizationRequest struct {
	Token string `json:"token"`
}

// AuthorizationResponse represents an authorization response
type AuthorizationResponse struct {
	Allowed bool   `json:"allowed"`
	Status  string `json:"status"`
	Score   string `json:"score,omitempty"`
	Cache   string `json:"cache,omitempty"`
}

// NewService creates a new authorization service
func NewService(cfg *config.Config) (*Service, error) {
	// Create reCAPTCHA client
	recaptchaConfig := &recaptcha.Config{
		ProjectID:   cfg.RecaptchaProjectID,
		SiteKey:     cfg.RecaptchaSiteKey,
		Action:      cfg.RecaptchaAction,
		V3Threshold: cfg.RecaptchaV3Threshold,
		Timeout:     time.Duration(cfg.GoogleAPITimeoutSeconds) * time.Second,
		MockMode:    cfg.MockMode,
	}
	recaptchaClient := recaptcha.NewClient(recaptchaConfig)

	// Create cache
	cacheConfig := cache.Config{
		Type:          "redis",
		RedisURL:      cfg.RedisURL,
		DefaultTTL:    time.Duration(cfg.CacheTTLSeconds) * time.Second,
		FailedTTL:     time.Duration(cfg.CacheFailedTTLSeconds) * time.Second,
		MaxMemorySize: 10000, // Not used for Redis
	}
	cacheInstance, err := cache.NewCache(cacheConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	// Create circuit breaker
	circuitBreakerConfig := circuitbreaker.Config{
		FailureThreshold:    cfg.CircuitBreakerFailureThreshold,
		RecoveryTime:        cfg.CircuitBreakerRecoveryTime,
		HalfOpenMaxRequests: 3, // Allow 3 requests in half-open state
	}
	circuitBreaker := circuitbreaker.NewBreaker(circuitBreakerConfig)

	// Create telemetry
	telemetryConfig := observability.Config{
		ServiceName:    cfg.OTelServiceName,
		ServiceVersion: "1.0.0",
		Environment:    "production",
		OTelEndpoint:   cfg.OTelEndpoint,
		LogLevel:       cfg.LogLevel,
	}
	telemetry, err := observability.NewTelemetry(telemetryConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create telemetry: %w", err)
	}

	// Create metrics
	var metrics *observability.Metrics
	if telemetry.Meter != nil {
		metrics, err = observability.NewMetrics(telemetry.Meter)
		if err != nil {
			return nil, fmt.Errorf("failed to create metrics: %w", err)
		}
	}

	return &Service{
		config:         cfg,
		recaptchaClient: recaptchaClient,
		cache:          cacheInstance,
		circuitBreaker: circuitBreaker,
		telemetry:      telemetry,
		metrics:        metrics,
	}, nil
}

// Authorize validates a reCAPTCHA token and returns an authorization decision
func (s *Service) Authorize(ctx context.Context, req *AuthorizationRequest) (*AuthorizationResponse, error) {
	startTime := time.Now()
	requestID := generateRequestID()

	// Create span for tracing
	ctx, span := s.telemetry.Tracer.Start(ctx, "authorize",
		trace.WithAttributes(
			attribute.String("request_id", requestID),
			attribute.Int("token_length", len(req.Token)),
		),
	)
	defer span.End()

	// Record metrics
	if s.metrics != nil {
		s.metrics.RequestsTotal.Add(ctx, 1)
		defer func() {
			s.metrics.ResponseTime.Record(ctx, time.Since(startTime).Seconds())
		}()
	}

	// Check cache first
	cacheKey := cache.GenerateCacheKey(req.Token)
			cachedResult, err := s.cache.Get(ctx, cacheKey)
		if err == nil && cachedResult != nil {
			// Cache hit
			if s.metrics != nil {
				s.metrics.CacheHits.Add(ctx, 1)
			}

			s.telemetry.LogCache("get", cacheKey, true, time.Since(startTime))

			// Convert cache.ValidationResult to recaptcha.ValidationResult
			recaptchaResult := s.convertCacheResult(cachedResult)
			response := s.createResponse(recaptchaResult, "hit")
			s.logRequest(requestID, req.Token, response.Status, true, time.Since(startTime), nil)
			return response, nil
		}

	// Cache miss
	if s.metrics != nil {
		s.metrics.CacheMisses.Add(ctx, 1)
	}

	s.telemetry.LogCache("get", cacheKey, false, time.Since(startTime))

	// Check circuit breaker
	if s.config.CircuitBreakerEnabled && s.circuitBreaker.IsOpen() {
		// Circuit breaker is open, handle based on failure mode
		response := s.handleCircuitBreakerOpen()
		s.logRequest(requestID, req.Token, response.Status, false, time.Since(startTime), nil)
		return response, nil
	}

	// Validate with Google API
	var validationResult *recaptcha.ValidationResult
	var validationErr error

	if s.config.CircuitBreakerEnabled {
		// Use circuit breaker
		validationErr = s.circuitBreaker.Execute(ctx, func() error {
			result, err := s.validateWithGoogle(ctx, req.Token)
			if err != nil {
				return err
			}
			validationResult = result
			return nil
		})
	} else {
		// Direct validation
		validationResult, validationErr = s.validateWithGoogle(ctx, req.Token)
	}

	// Handle validation result
	if validationErr != nil {
		// Validation failed
		if s.metrics != nil {
			s.metrics.ErrorsTotal.Add(ctx, 1)
		}

		response := s.handleValidationError(validationErr)
		s.logRequest(requestID, req.Token, response.Status, false, time.Since(startTime), validationErr)
		return response, nil
	}

	// Cache the result
	s.cacheResult(ctx, cacheKey, validationResult)

	// Create response
	response := s.createResponse(validationResult, "miss")
	s.logRequest(requestID, req.Token, response.Status, false, time.Since(startTime), nil)

	return response, nil
}

// validateWithGoogle validates the token with Google's reCAPTCHA API
func (s *Service) validateWithGoogle(ctx context.Context, token string) (*recaptcha.ValidationResult, error) {
	ctx, span := s.telemetry.Tracer.Start(ctx, "validate_with_google")
	defer span.End()

	startTime := time.Now()
	result, err := s.recaptchaClient.Validate(ctx, token)
	duration := time.Since(startTime)

	// Record metrics
	if s.metrics != nil {
		s.metrics.GoogleAPIDuration.Record(ctx, duration.Seconds())
		if err == nil && result.IsValidToken() {
			s.metrics.ValidationSuccess.Add(ctx, 1)
		} else {
			s.metrics.ValidationFailure.Add(ctx, 1)
		}
	}

	// Log validation
	s.telemetry.LogValidation(
		"", // requestID will be set by caller
		token,
		result.IsValidToken(),
		result.GetScore(),
		result.ErrorCodes,
		duration,
	)

	return result, err
}

// cacheResult caches the validation result
func (s *Service) cacheResult(ctx context.Context, key string, result *recaptcha.ValidationResult) {
	// Convert to cache format
	cacheResult := &cache.ValidationResult{
		Success:     result.Success,
		Score:       result.Score,
		Action:      result.Action,
		ChallengeTS: result.ChallengeTS,
		Hostname:    result.Hostname,
		ErrorCodes:  result.ErrorCodes,
		Timestamp:   time.Now(),
	}

	// Determine TTL based on result
	ttl := time.Duration(s.config.CacheTTLSeconds) * time.Second
	if !result.IsValidToken() {
		ttl = time.Duration(s.config.CacheFailedTTLSeconds) * time.Second
	}

	// Cache the result
	if err := s.cache.Set(ctx, key, cacheResult, ttl); err != nil {
		s.telemetry.Logger.WithError(err).Warn("Failed to cache validation result")
	}
}

// createResponse creates an authorization response
func (s *Service) createResponse(result *recaptcha.ValidationResult, cacheStatus string) *AuthorizationResponse {
	response := &AuthorizationResponse{
		Allowed: result.IsValidToken(),
		Status:  "valid",
		Cache:   cacheStatus,
	}

	if !result.IsValidToken() {
		response.Status = "invalid"
		if len(result.ErrorCodes) > 0 {
			response.Status = result.ErrorCodes[0]
		}
	}

	if result.Score > 0 {
		response.Score = strconv.FormatFloat(result.Score, 'f', 2, 64)
	}

	return response
}

// handleCircuitBreakerOpen handles requests when circuit breaker is open
func (s *Service) handleCircuitBreakerOpen() *AuthorizationResponse {
	if s.config.FailureMode == "fail_open" {
		return &AuthorizationResponse{
			Allowed: true,
			Status:  "degraded",
			Cache:   "miss",
		}
	}

	return &AuthorizationResponse{
		Allowed: false,
		Status:  "circuit_breaker_open",
		Cache:   "miss",
	}
}

// handleValidationError handles validation errors
func (s *Service) handleValidationError(err error) *AuthorizationResponse {
	if s.config.FailureMode == "fail_open" {
		return &AuthorizationResponse{
			Allowed: true,
			Status:  "degraded",
			Cache:   "miss",
		}
	}

	return &AuthorizationResponse{
		Allowed: false,
		Status:  "timeout",
		Cache:   "miss",
	}
}

// logRequest logs the request with telemetry
func (s *Service) logRequest(requestID, token, status string, cacheHit bool, responseTime time.Duration, err error) {
	s.telemetry.LogRequest(observability.LogFields{
		RequestID:     requestID,
		Token:         token,
		ValidationResult: status,
		CacheHit:      cacheHit,
		ResponseTime:  responseTime,
		Error:         err,
		CircuitBreakerState: s.circuitBreaker.GetStateString(),
	})
}

// GetHealth returns the health status of the service
func (s *Service) GetHealth() map[string]interface{} {
	stats := s.circuitBreaker.GetStats()
	cacheStats := s.cache.GetStats()

	return map[string]interface{}{
		"status": "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"circuit_breaker": map[string]interface{}{
			"state":           stats.State,
			"failure_count":   stats.FailureCount,
			"total_requests":  stats.TotalRequests,
			"total_failures":  stats.TotalFailures,
		},
		"cache": map[string]interface{}{
			"hits":   cacheStats.Hits,
			"misses": cacheStats.Misses,
			"size":   cacheStats.Size,
		},
		"config": map[string]interface{}{
			"recaptcha_project_id": s.config.RecaptchaProjectID,
			"recaptcha_action":     s.config.RecaptchaAction,
			"failure_mode":         s.config.FailureMode,
			"mock_mode":            s.config.MockMode,
		},
	}
}

// GetMetrics returns the current metrics
func (s *Service) GetMetrics() map[string]interface{} {
	stats := s.circuitBreaker.GetStats()
	cacheStats := s.cache.GetStats()

	return map[string]interface{}{
		"circuit_breaker": stats,
		"cache":          cacheStats,
	}
}

// Shutdown gracefully shuts down the service
func (s *Service) Shutdown(ctx context.Context) error {
	return s.telemetry.Shutdown(ctx)
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// convertCacheResult converts cache.ValidationResult to recaptcha.ValidationResult
func (s *Service) convertCacheResult(cachedResult *cache.ValidationResult) *recaptcha.ValidationResult {
	return &recaptcha.ValidationResult{
		Success:     cachedResult.Success,
		Score:       cachedResult.Score,
		Action:      cachedResult.Action,
		ChallengeTS: cachedResult.ChallengeTS,
		Hostname:    cachedResult.Hostname,
		ErrorCodes:  cachedResult.ErrorCodes,
	}
} 