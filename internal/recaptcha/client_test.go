package recaptcha

import (
	"context"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	config := &Config{
		ProjectID:   "test-project",
		SiteKey:     "test_site_key",
		Action:      "authz",
		V3Threshold: 0.5,
		Timeout:     5 * time.Second,
		MockMode:    true,
	}

	client := NewClient(config)
	if client == nil {
		t.Fatal("Expected client to be created")
	}
}

func TestClient_Validate_MockMode(t *testing.T) {
	tests := []struct {
		name           string
		token          string
		expectedValid  bool
		expectedScore  float64
		expectedError  bool
	}{
		{
			name:          "valid token",
			token:         "valid_token",
			expectedValid: true,
			expectedScore: 0.9,
			expectedError: false,
		},
		{
			name:          "invalid token",
			token:         "invalid_token",
			expectedValid: false,
			expectedScore: 0,
			expectedError: false,
		},
		{
			name:          "low score token",
			token:         "low_score_token",
			expectedValid: false,
			expectedScore: 0.1,
			expectedError: false,
		},
		{
			name:          "timeout token",
			token:         "timeout_token",
			expectedValid: false,
			expectedScore: 0,
			expectedError: true,
		},
		{
			name:          "error token",
			token:         "error_token",
			expectedValid: false,
			expectedScore: 0,
			expectedError: false,
		},
		{
			name:          "empty token",
			token:         "",
			expectedValid: false,
			expectedScore: 0,
			expectedError: false,
		},
		{
			name:          "random token",
			token:         "random_token_123",
			expectedValid: true,
			expectedScore: 0.8,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
					config := &Config{
			ProjectID:   "test-project",
			SiteKey:     "test_site_key",
			Action:      "authz",
			V3Threshold: 0.5,
			Timeout:     5 * time.Second,
			MockMode:    true,
		}

			client := NewClient(config)
			ctx := context.Background()

			result, err := client.Validate(ctx, tt.token)

			if tt.expectedError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result.Success != tt.expectedValid {
				t.Errorf("Expected valid=%v, got %v", tt.expectedValid, result.Success)
			}

			if result.Score != tt.expectedScore {
				t.Errorf("Expected score=%v, got %v", tt.expectedScore, result.Score)
			}
		})
	}
}

func TestValidationResult_IsValidToken(t *testing.T) {
	tests := []struct {
		name     string
		result   *ValidationResult
		expected bool
	}{
		{
			name: "valid token",
			result: &ValidationResult{
				Success:    true,
				ErrorCodes: []string{},
			},
			expected: true,
		},
		{
			name: "invalid token",
			result: &ValidationResult{
				Success:    false,
				ErrorCodes: []string{"invalid-input-response"},
			},
			expected: false,
		},
		{
			name: "success but with error codes",
			result: &ValidationResult{
				Success:    true,
				ErrorCodes: []string{"some-error"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.IsValidToken(); got != tt.expected {
				t.Errorf("IsValidToken() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestValidationResult_GetScore(t *testing.T) {
	result := &ValidationResult{
		Score: 0.85,
	}

	if got := result.GetScore(); got != 0.85 {
		t.Errorf("GetScore() = %v, want %v", got, 0.85)
	}
}

func TestValidationResult_GetErrorCodes(t *testing.T) {
	tests := []struct {
		name     string
		result   *ValidationResult
		expected string
	}{
		{
			name: "no error codes",
			result: &ValidationResult{
				ErrorCodes: []string{},
			},
			expected: "",
		},
		{
			name: "single error code",
			result: &ValidationResult{
				ErrorCodes: []string{"invalid-input-response"},
			},
			expected: "invalid-input-response",
		},
		{
			name: "multiple error codes",
			result: &ValidationResult{
				ErrorCodes: []string{"timeout-or-duplicate", "invalid-input-response"},
			},
			expected: "timeout-or-duplicate, invalid-input-response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.GetErrorCodes(); got != tt.expected {
				t.Errorf("GetErrorCodes() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestValidationResult_String(t *testing.T) {
	tests := []struct {
		name     string
		result   *ValidationResult
		expected string
	}{
		{
			name: "valid token without score",
			result: &ValidationResult{
				Success: true,
			},
			expected: "valid",
		},
		{
			name: "valid token with score",
			result: &ValidationResult{
				Success: true,
				Score:   0.9,
			},
			expected: "valid (score: 0.90)",
		},
		{
			name: "invalid token",
			result: &ValidationResult{
				Success:    false,
				ErrorCodes: []string{"invalid-input-response"},
			},
			expected: "invalid (invalid-input-response)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.String(); got != tt.expected {
				t.Errorf("String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestClient_Validate_V3Threshold(t *testing.T) {
	config := &Config{
		ProjectID:   "test-project",
		SiteKey:     "test_site_key",
		Action:      "authz",
		V3Threshold: 0.7, // High threshold
		Timeout:     5 * time.Second,
		MockMode:    true,
	}

	client := NewClient(config)
	ctx := context.Background()

	// Test with a token that would normally be valid but has low score
	result, err := client.Validate(ctx, "low_score_token")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should be invalid due to threshold
	if result.Success {
		t.Error("Expected validation to fail due to threshold")
	}

	// Should have score-below-threshold error
	found := false
	for _, code := range result.ErrorCodes {
		if code == "score-below-threshold" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected score-below-threshold error code")
	}
}

func TestClient_Validate_EmptyToken(t *testing.T) {
	config := &Config{
		ProjectID:   "test-project",
		SiteKey:     "test_site_key",
		Action:      "authz",
		V3Threshold: 0.5,
		Timeout:     5 * time.Second,
		MockMode:    false, // Test with real mode
	}

	client := NewClient(config)
	ctx := context.Background()

	result, err := client.Validate(ctx, "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result.Success {
		t.Error("Expected empty token to be invalid")
	}

	found := false
	for _, code := range result.ErrorCodes {
		if code == "missing-input-response" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected missing-input-response error code")
	}
} 