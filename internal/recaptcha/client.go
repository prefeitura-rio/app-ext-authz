package recaptcha

import (
	"context"
	"fmt"
	"strings"
	"time"

	recaptcha "cloud.google.com/go/recaptchaenterprise/v2/apiv1"
	recaptchapb "cloud.google.com/go/recaptchaenterprise/v2/apiv1/recaptchaenterprisepb"
)

// Client handles reCAPTCHA validation
type Client interface {
	Validate(ctx context.Context, token string) (*ValidationResult, error)
}

// ValidationResult represents the result of a reCAPTCHA validation
type ValidationResult struct {
	Success     bool    `json:"success"`
	Score       float64 `json:"score,omitempty"`       // Only for v3
	Action      string  `json:"action,omitempty"`      // Only for v3
	ChallengeTS string  `json:"challenge_ts,omitempty"`
	Hostname    string  `json:"hostname,omitempty"`
	ErrorCodes  []string `json:"error-codes,omitempty"`
}



// Config holds client configuration
type Config struct {
	ProjectID    string
	SiteKey      string
	Action       string
	V3Threshold  float64
	Timeout      time.Duration
	MockMode     bool
}

// client implements the Client interface
type client struct {
	config *Config
	client *recaptcha.Client
}

// NewClient creates a new reCAPTCHA client
func NewClient(config *Config) Client {
	ctx := context.Background()
	recaptchaClient, err := recaptcha.NewClient(ctx)
	if err != nil {
		// In mock mode, we can continue without a real client
		if config.MockMode {
			return &client{
				config: config,
				client: nil,
			}
		}
		panic(fmt.Sprintf("failed to create reCAPTCHA client: %v", err))
	}

	return &client{
		config: config,
		client: recaptchaClient,
	}
}

// Validate validates a reCAPTCHA token using Google Cloud reCAPTCHA Enterprise
func (c *client) Validate(ctx context.Context, token string) (*ValidationResult, error) {
	if c.config.MockMode {
		return c.mockValidation(token)
	}

	if token == "" {
		return &ValidationResult{
			Success:    false,
			ErrorCodes: []string{"missing-input-response"},
		}, nil
	}

	// Create assessment request
	event := &recaptchapb.Event{
		Token:   token,
		SiteKey: c.config.SiteKey,
	}

	assessment := &recaptchapb.Assessment{
		Event: event,
	}

	request := &recaptchapb.CreateAssessmentRequest{
		Assessment: assessment,
		Parent:     fmt.Sprintf("projects/%s", c.config.ProjectID),
	}

	// Create assessment
	response, err := c.client.CreateAssessment(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to create assessment: %w", err)
	}

	// Check if token is valid
	if !response.TokenProperties.Valid {
		errorCodes := []string{}
		switch response.TokenProperties.InvalidReason {
		case recaptchapb.TokenProperties_INVALID_REASON_UNSPECIFIED:
			errorCodes = append(errorCodes, "invalid-reason-unspecified")
		case recaptchapb.TokenProperties_UNKNOWN_INVALID_REASON:
			errorCodes = append(errorCodes, "unknown-invalid-reason")
		case recaptchapb.TokenProperties_MALFORMED:
			errorCodes = append(errorCodes, "malformed")
		case recaptchapb.TokenProperties_EXPIRED:
			errorCodes = append(errorCodes, "expired")
		case recaptchapb.TokenProperties_DUPE:
			errorCodes = append(errorCodes, "dupe")
		case recaptchapb.TokenProperties_MISSING:
			errorCodes = append(errorCodes, "missing")
		case recaptchapb.TokenProperties_BROWSER_ERROR:
			errorCodes = append(errorCodes, "browser-error")
		}

		return &ValidationResult{
			Success:    false,
			ErrorCodes: errorCodes,
		}, nil
	}

	// Check if action matches expected action
	if response.TokenProperties.Action != c.config.Action {
		return &ValidationResult{
			Success:    false,
			ErrorCodes: []string{"action-mismatch"},
		}, nil
	}

	// Get risk analysis
	score := float64(response.RiskAnalysis.Score)
	success := score >= c.config.V3Threshold

	result := &ValidationResult{
		Success:     success,
		Score:       score,
		Action:      response.TokenProperties.Action,
		ChallengeTS: response.TokenProperties.CreateTime.AsTime().Format(time.RFC3339),
		Hostname:    response.TokenProperties.Hostname,
		ErrorCodes:  []string{},
	}

	// If score is below threshold, add error code
	if !success {
		result.ErrorCodes = append(result.ErrorCodes, "score-below-threshold")
	}

	return result, nil
}

// mockValidation provides mock responses for testing
func (c *client) mockValidation(token string) (*ValidationResult, error) {
	// Mock different scenarios based on token
	switch token {
	case "valid_token":
		return &ValidationResult{
			Success:     true,
			Score:       0.9,
			Action:      c.config.Action,
			ChallengeTS: time.Now().Format(time.RFC3339),
			Hostname:    "localhost",
		}, nil

	case "invalid_token":
		return &ValidationResult{
			Success:    false,
			ErrorCodes: []string{"malformed"},
		}, nil

	case "low_score_token":
		return &ValidationResult{
			Success:     false,
			Score:       0.1,
			Action:      c.config.Action,
			ChallengeTS: time.Now().Format(time.RFC3339),
			Hostname:    "localhost",
			ErrorCodes:  []string{"score-below-threshold"},
		}, nil

	case "timeout_token":
		// Simulate timeout
		time.Sleep(c.config.Timeout + time.Second)
		return nil, fmt.Errorf("timeout")

	case "error_token":
		return &ValidationResult{
			Success:    false,
			ErrorCodes: []string{"expired"},
		}, nil

	case "":
		// Empty token should be invalid
		return &ValidationResult{
			Success:    false,
			ErrorCodes: []string{"missing-input-response"},
		}, nil

	default:
		// For any other token, simulate a valid response
		return &ValidationResult{
			Success:     true,
			Score:       0.8,
			Action:      c.config.Action,
			ChallengeTS: time.Now().Format(time.RFC3339),
			Hostname:    "localhost",
		}, nil
	}
}

// IsValidToken checks if a token is valid based on the validation result
func (r *ValidationResult) IsValidToken() bool {
	if r == nil {
		return false
	}
	return r.Success && len(r.ErrorCodes) == 0
}

// GetScore returns the score for v3 validation
func (r *ValidationResult) GetScore() float64 {
	if r == nil {
		return 0.0
	}
	return r.Score
}

// GetErrorCodes returns the error codes as a string
func (r *ValidationResult) GetErrorCodes() string {
	if r == nil {
		return "nil-result"
	}
	if len(r.ErrorCodes) == 0 {
		return ""
	}
	return strings.Join(r.ErrorCodes, ", ")
}

// String returns a string representation of the validation result
func (r *ValidationResult) String() string {
	if r.Success {
		if r.Score > 0 {
			return fmt.Sprintf("valid (score: %.2f)", r.Score)
		}
		return "valid"
	}
	return fmt.Sprintf("invalid (%s)", r.GetErrorCodes())
} 