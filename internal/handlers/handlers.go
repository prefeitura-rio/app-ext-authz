package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prefeitura-rio/app-ext-authz/internal/service"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Context key type for request ID
type contextKey string

const requestIDKey contextKey = "request_id"

// Handler handles HTTP requests
type Handler struct {
	service *service.Service
}

// NewHandler creates a new HTTP handler
func NewHandler(svc *service.Service) *Handler {
	return &Handler{
		service: svc,
	}
}

// RegisterRoutes registers all HTTP routes
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	// Middleware
	r.Use(gin.Recovery())
	r.Use(h.corsMiddleware())
	r.Use(h.requestIDMiddleware())
	r.Use(h.loggingMiddleware())

	// Health check
	r.GET("/health", h.healthHandler)

	// Metrics
	r.GET("/metrics", h.metricsHandler)

	// Authorization endpoint
	r.POST("/authz", h.authorizationHandler)

	// Root endpoint
	r.GET("/", h.rootHandler)
}

// authorizationHandler handles authorization requests
func (h *Handler) authorizationHandler(c *gin.Context) {
	ctx := c.Request.Context()
	startTime := time.Now()

	// Extract token from header
	token := c.GetHeader("X-Recaptcha-Token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "X-Recaptcha-Token header is required",
		})
		return
	}

	// Create authorization request
	req := &service.AuthorizationRequest{
		Token: token,
	}

	// Call service
	response, err := h.service.Authorize(ctx, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
		})
		return
	}

	// Set response headers
	c.Header("X-Recaptcha-Status", response.Status)
	if response.Score != "" {
		c.Header("X-Recaptcha-Score", response.Score)
	}
	c.Header("X-Recaptcha-Cache", response.Cache)

	// Return response
	if response.Allowed {
		c.Status(http.StatusOK)
	} else {
		c.Status(http.StatusForbidden)
	}

	// Log request
	h.logRequest(c, startTime, response, err)
}

// healthHandler handles health check requests
func (h *Handler) healthHandler(c *gin.Context) {
	health := h.service.GetHealth()
	c.JSON(http.StatusOK, health)
}

// metricsHandler handles metrics requests
func (h *Handler) metricsHandler(c *gin.Context) {
	metrics := h.service.GetMetrics()
	c.JSON(http.StatusOK, metrics)
}

// rootHandler handles root requests
func (h *Handler) rootHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service": "recaptcha-authz",
		"version": "1.0.0",
		"status":  "running",
		"endpoints": gin.H{
			"authorization": "/authz",
			"health":        "/health",
			"metrics":       "/metrics",
		},
	})
}

// corsMiddleware adds CORS headers
func (h *Handler) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, X-Recaptcha-Token")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// requestIDMiddleware adds request ID to context
func (h *Handler) requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)

		// Add to context for tracing
		ctx := context.WithValue(c.Request.Context(), requestIDKey, requestID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// loggingMiddleware logs requests
func (h *Handler) loggingMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		// Custom JSON logging
		logData := map[string]interface{}{
			"timestamp":     param.TimeStamp.Format(time.RFC3339),
			"method":        param.Method,
			"path":          param.Path,
			"status":        param.StatusCode,
			"latency":       param.Latency.String(),
			"client_ip":     param.ClientIP,
			"user_agent":    param.Request.UserAgent(),
			"request_id":    param.Keys["request_id"],
			"error_message": param.ErrorMessage,
		}

		jsonData, _ := json.Marshal(logData)
		return string(jsonData) + "\n"
	})
}

// logRequest logs the request with tracing
func (h *Handler) logRequest(c *gin.Context, startTime time.Time, response *service.AuthorizationResponse, err error) {
	ctx := c.Request.Context()
	requestID := c.GetString("request_id")
	token := c.GetHeader("X-Recaptcha-Token")

	// Add span attributes
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		span.SetAttributes(
			attribute.String("http.method", c.Request.Method),
			attribute.String("http.path", c.Request.URL.Path),
			attribute.Int("http.status_code", c.Writer.Status()),
			attribute.String("recaptcha.status", response.Status),
			attribute.String("recaptcha.cache", response.Cache),
			attribute.Bool("recaptcha.allowed", response.Allowed),
		)
	}

	// Log with structured fields
	logFields := map[string]interface{}{
		"request_id":     requestID,
		"method":         c.Request.Method,
		"path":           c.Request.URL.Path,
		"status_code":    c.Writer.Status(),
		"response_time":  time.Since(startTime).String(),
		"token_length":   len(token),
		"recaptcha_status": response.Status,
		"recaptcha_cache":  response.Cache,
		"recaptcha_allowed": response.Allowed,
	}

	if response.Score != "" {
		logFields["recaptcha_score"] = response.Score
	}

	if err != nil {
		logFields["error"] = err.Error()
	}

	// Use gin's logger or your own logging
	// For now, we'll use fmt.Printf for simplicity
	logJSON, _ := json.Marshal(logFields)
	fmt.Printf("%s\n", string(logJSON))
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}



 