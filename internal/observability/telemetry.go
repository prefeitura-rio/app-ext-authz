package observability

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// Telemetry holds all observability components
type Telemetry struct {
	Tracer   trace.Tracer
	Meter    metric.Meter
	Logger   *logrus.Logger
	Provider *sdktrace.TracerProvider
}

// Config holds telemetry configuration
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTelEndpoint   string
	LogLevel       string
}

// NewTelemetry creates a new telemetry instance
func NewTelemetry(config Config) (*Telemetry, error) {
	// Setup logger
	logger := logrus.New()
	level, err := logrus.ParseLevel(config.LogLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	logger.SetLevel(level)
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})

	// Create resource
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(config.ServiceVersion),
			semconv.DeploymentEnvironment(config.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	var provider *sdktrace.TracerProvider

	// Setup tracing if endpoint is provided
	if config.OTelEndpoint != "" {
		traceExporter, err := otlptracehttp.New(
			context.Background(),
			otlptracehttp.WithEndpoint(config.OTelEndpoint),
			otlptracehttp.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create trace exporter: %w", err)
		}

		provider = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExporter),
			sdktrace.WithResource(res),
		)

		otel.SetTracerProvider(provider)
	}

	// Setup metrics if endpoint is provided
	var meter metric.Meter
	if config.OTelEndpoint != "" {
		// Create metric exporter
		metricExporter, err := otlpmetrichttp.New(
			context.Background(),
			otlpmetrichttp.WithEndpoint(config.OTelEndpoint),
			otlpmetrichttp.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create metric exporter: %w", err)
		}

		// Create meter provider with the exporter
		meterProvider := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		)
		
		// Set as global meter provider
		otel.SetMeterProvider(meterProvider)
		
		// Get meter from the provider
		meter = meterProvider.Meter(config.ServiceName)
	}

	// Create tracer
	tracer := otel.Tracer(config.ServiceName)

	return &Telemetry{
		Tracer:   tracer,
		Meter:    meter,
		Logger:   logger,
		Provider: provider,
	}, nil
}

// Shutdown gracefully shuts down telemetry
func (t *Telemetry) Shutdown(ctx context.Context) error {
	if t.Provider != nil {
		return t.Provider.Shutdown(ctx)
	}
	return nil
}

// Metrics holds all the metrics
type Metrics struct {
	RequestsTotal           metric.Int64Counter
	ValidationSuccess       metric.Int64Counter
	ValidationFailure       metric.Int64Counter
	CacheHits               metric.Int64Counter
	CacheMisses             metric.Int64Counter
	GoogleAPIDuration       metric.Float64Histogram
	CircuitBreakerState     metric.Int64UpDownCounter
	CircuitBreakerTrips     metric.Int64Counter
	ResponseTime            metric.Float64Histogram
	ErrorsTotal             metric.Int64Counter
}

// NewMetrics creates new metrics
func NewMetrics(meter metric.Meter) (*Metrics, error) {
	requestsTotal, err := meter.Int64Counter(
		"recaptcha_requests_total",
		metric.WithDescription("Total number of requests processed"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create requests counter: %w", err)
	}

	validationSuccess, err := meter.Int64Counter(
		"recaptcha_validation_success_total",
		metric.WithDescription("Total number of successful validations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create validation success counter: %w", err)
	}

	validationFailure, err := meter.Int64Counter(
		"recaptcha_validation_failure_total",
		metric.WithDescription("Total number of failed validations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create validation failure counter: %w", err)
	}

	cacheHits, err := meter.Int64Counter(
		"recaptcha_cache_hits_total",
		metric.WithDescription("Total number of cache hits"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache hits counter: %w", err)
	}

	cacheMisses, err := meter.Int64Counter(
		"recaptcha_cache_misses_total",
		metric.WithDescription("Total number of cache misses"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache misses counter: %w", err)
	}

	googleAPIDuration, err := meter.Float64Histogram(
		"recaptcha_google_api_duration_seconds",
		metric.WithDescription("Duration of Google API calls"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Google API duration histogram: %w", err)
	}

	circuitBreakerState, err := meter.Int64UpDownCounter(
		"recaptcha_circuit_breaker_state",
		metric.WithDescription("Current state of circuit breaker (0=closed, 1=half-open, 2=open)"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create circuit breaker state counter: %w", err)
	}

	circuitBreakerTrips, err := meter.Int64Counter(
		"recaptcha_circuit_breaker_trips_total",
		metric.WithDescription("Total number of circuit breaker trips"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create circuit breaker trips counter: %w", err)
	}

	responseTime, err := meter.Float64Histogram(
		"recaptcha_response_time_seconds",
		metric.WithDescription("Response time of authorization requests"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create response time histogram: %w", err)
	}

	errorsTotal, err := meter.Int64Counter(
		"recaptcha_errors_total",
		metric.WithDescription("Total number of errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create errors counter: %w", err)
	}

	return &Metrics{
		RequestsTotal:       requestsTotal,
		ValidationSuccess:   validationSuccess,
		ValidationFailure:   validationFailure,
		CacheHits:           cacheHits,
		CacheMisses:         cacheMisses,
		GoogleAPIDuration:   googleAPIDuration,
		CircuitBreakerState: circuitBreakerState,
		CircuitBreakerTrips: circuitBreakerTrips,
		ResponseTime:        responseTime,
		ErrorsTotal:         errorsTotal,
	}, nil
}

// LogFields provides common log fields
type LogFields struct {
	RequestID     string
	Token         string
	ValidationResult string
	CacheHit      bool
	ResponseTime  time.Duration
	Error         error
	CircuitBreakerState string
}

// LogRequest logs a request with structured fields
func (t *Telemetry) LogRequest(fields LogFields) {
	logFields := logrus.Fields{
		"request_id":     fields.RequestID,
		"token_length":   len(fields.Token),
		"validation_result": fields.ValidationResult,
		"cache_hit":      fields.CacheHit,
		"response_time_ms": fields.ResponseTime.Milliseconds(),
		"circuit_breaker_state": fields.CircuitBreakerState,
	}

	if fields.Error != nil {
		logFields["error"] = fields.Error.Error()
		t.Logger.WithFields(logFields).Error("Request failed")
	} else {
		t.Logger.WithFields(logFields).Info("Request processed")
	}
}

// LogValidation logs validation details
func (t *Telemetry) LogValidation(requestID, token string, success bool, score float64, errorCodes []string, duration time.Duration) {
	logFields := logrus.Fields{
		"request_id":     requestID,
		"token_length":   len(token),
		"success":        success,
		"duration_ms":    duration.Milliseconds(),
	}

	if score > 0 {
		logFields["score"] = score
	}

	if len(errorCodes) > 0 {
		logFields["error_codes"] = errorCodes
	}

	if success {
		t.Logger.WithFields(logFields).Info("Validation successful")
	} else {
		t.Logger.WithFields(logFields).Warn("Validation failed")
	}
}

// LogCircuitBreaker logs circuit breaker state changes
func (t *Telemetry) LogCircuitBreaker(oldState, newState string, reason string) {
	t.Logger.WithFields(logrus.Fields{
		"old_state": oldState,
		"new_state": newState,
		"reason":    reason,
	}).Info("Circuit breaker state changed")
}

// LogCache logs cache operations
func (t *Telemetry) LogCache(operation, key string, hit bool, duration time.Duration) {
	t.Logger.WithFields(logrus.Fields{
		"operation":     operation,
		"key_length":    len(key),
		"hit":           hit,
		"duration_ms":   duration.Milliseconds(),
	}).Debug("Cache operation")
} 