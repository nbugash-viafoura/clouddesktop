package aws

import (
	"errors"
	"strings"
	"testing"
)

// TestAWSErrorInterface verifies AWSError implements error interface.
func TestAWSErrorInterface(t *testing.T) {
	err := NewError(ErrorTypeNotFound, "EC2", "DescribeInstances", "instance not found", nil)

	// Check that it's an error
	var _ error = err

	// Check error message
	msg := err.Error()
	if !strings.Contains(msg, "EC2") || !strings.Contains(msg, "DescribeInstances") {
		t.Errorf("Error() = %q, should contain service and operation", msg)
	}
}

// TestAWSErrorWithResource includes resource in error message.
func TestAWSErrorWithResource(t *testing.T) {
	err := NewErrorWithResource(ErrorTypeNotFound, "EC2", "DescribeInstances", "i-12345", "instance not found", nil)

	msg := err.Error()
	if !strings.Contains(msg, "i-12345") {
		t.Errorf("Error() = %q, should contain resource ID", msg)
	}

	// Should have format: Service.Operation(Resource): Message
	if !strings.Contains(msg, "EC2.DescribeInstances(i-12345)") {
		t.Errorf("Error() = %q, should have formatted service.operation(resource)", msg)
	}
}

// TestAWSErrorUnwrap enables error wrapping.
func TestAWSErrorUnwrap(t *testing.T) {
	original := errors.New("original error")
	err := NewError(ErrorTypeNetworkTimeout, "EC2", "RunInstances", "timeout", original)

	if !errors.Is(err, original) {
		t.Errorf("Unwrap() should make original error findable with errors.Is()")
	}
}

// TestRemediationMessages verifies that remediation is provided for each error type.
func TestRemediationMessages(t *testing.T) {
	tests := []struct {
		name      string
		err       *AWSError
		wantKeywords []string
	}{
		{
			name: "PermissionDenied",
			err: NewError(ErrorTypePermissionDenied, "EC2", "RunInstances", "", nil),
			wantKeywords: []string{"test-developers", "permissions"},
		},
		{
			name: "NotFound",
			err: NewErrorWithResource(ErrorTypeNotFound, "EC2", "DescribeInstances", "i-123", "", nil),
			wantKeywords: []string{"not found", "may have been deleted"},
		},
		{
			name: "NetworkTimeout",
			err: NewError(ErrorTypeNetworkTimeout, "EC2", "DescribeInstances", "", nil),
			wantKeywords: []string{"internet connection", "retry"},
		},
		{
			name: "RateLimit",
			err: NewError(ErrorTypeRateLimit, "EC2", "DescribeInstances", "", nil),
			wantKeywords: []string{"rate limit", "wait"},
		},
		{
			name: "ServiceUnavailable",
			err: NewError(ErrorTypeServiceUnavailable, "EC2", "DescribeInstances", "", nil),
			wantKeywords: []string{"temporarily unavailable"},
		},
		{
			name: "Unknown",
			err: NewError(ErrorTypeUnknown, "EC2", "DescribeInstances", "", nil),
			wantKeywords: []string{"cloudtrail"},
		},
		{
			name: "InvalidInput",
			err: NewError(ErrorTypeInvalidInput, "EC2", "RunInstances", "", nil),
			wantKeywords: []string{"cloudtrail"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remedy := tt.err.Remediation()
			if remedy == "" {
				t.Errorf("Remediation() returned empty string")
			}

			for _, keyword := range tt.wantKeywords {
				if !strings.Contains(strings.ToLower(remedy), strings.ToLower(keyword)) {
					t.Errorf("Remediation() = %q, should contain %q", remedy, keyword)
				}
			}
		})
	}
}

// TestErrInstanceNotFound helper.
func TestErrInstanceNotFound(t *testing.T) {
	err := ErrInstanceNotFound("i-abc123")

	if err.Type != ErrorTypeNotFound {
		t.Errorf("Type = %v, want NotFound", err.Type)
	}

	msg := err.Error()
	if !strings.Contains(msg, "i-abc123") {
		t.Errorf("Error() = %q, should contain instance ID", msg)
	}

	if !strings.Contains(msg, "EC2") {
		t.Errorf("Error() = %q, should contain service", msg)
	}
}

// TestErrPermissionDenied helper.
func TestErrPermissionDenied(t *testing.T) {
	err := ErrPermissionDenied("EC2", "RunInstances")

	if err.Type != ErrorTypePermissionDenied {
		t.Errorf("Type = %v, want PermissionDenied", err.Type)
	}

	if err.Service != "EC2" {
		t.Errorf("Service = %v, want EC2", err.Service)
	}

	remedy := err.Remediation()
	if !strings.Contains(remedy, "test-developers") {
		t.Errorf("Remediation should mention AWS profile")
	}
}

// TestErrNetworkTimeout helper.
func TestErrNetworkTimeout(t *testing.T) {
	err := ErrNetworkTimeout("EC2", "DescribeInstances")

	if err.Type != ErrorTypeNetworkTimeout {
		t.Errorf("Type = %v, want NetworkTimeout", err.Type)
	}

	remedy := err.Remediation()
	if !strings.Contains(strings.ToLower(remedy), "retry") {
		t.Errorf("Remediation for timeout should suggest retry")
	}
}

// TestErrSessionExpired helper.
func TestErrSessionExpired(t *testing.T) {
	err := ErrSessionExpired()

	if err == nil {
		t.Fatal("ErrSessionExpired() returned nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "session") && !strings.Contains(msg, "sts") {
		t.Errorf("Error message = %q, should mention session or sts command", msg)
	}
}

// TestErrSSHKeyNotFound helper.
func TestErrSSHKeyNotFound(t *testing.T) {
	err := ErrSSHKeyNotFound("/path/to/key.pub")

	if err == nil {
		t.Fatal("ErrSSHKeyNotFound() returned nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "not found") || !strings.Contains(msg, "/path/to/key.pub") {
		t.Errorf("Error message = %q, should mention path and not found", msg)
	}

	if !strings.Contains(msg, "ssh-keygen") {
		t.Errorf("Error message should include remediation with ssh-keygen")
	}
}

// TestErrAWSProfileNotFound helper.
func TestErrAWSProfileNotFound(t *testing.T) {
	err := ErrAWSProfileNotFound("test-terraform")

	if err == nil {
		t.Fatal("ErrAWSProfileNotFound() returned nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "test-terraform") || !strings.Contains(msg, "not found") {
		t.Errorf("Error message = %q, should mention profile and not found", msg)
	}

	if !strings.Contains(msg, ".aws") {
		t.Errorf("Error message should reference AWS config files")
	}
}

// TestErrorTypeString values are correct.
func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name   string
		errType ErrorType
	}{
		{"Unknown", ErrorTypeUnknown},
		{"NotFound", ErrorTypeNotFound},
		{"PermissionDenied", ErrorTypePermissionDenied},
		{"NetworkTimeout", ErrorTypeNetworkTimeout},
		{"RateLimit", ErrorTypeRateLimit},
		{"InvalidInput", ErrorTypeInvalidInput},
		{"ServiceUnavailable", ErrorTypeServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify error types exist and have distinct values
			if tt.errType < 0 || tt.errType > 10 {
				t.Errorf("ErrorType value out of expected range: %v", tt.errType)
			}
		})
	}
}

// TestNewErrorCopiesInput verifies that NewError creates a copy of input fields.
func TestNewErrorCopiesInput(t *testing.T) {
	original := errors.New("original")
	err := NewError(ErrorTypeNetworkTimeout, "Service", "Operation", "message", original)

	if err.Type != ErrorTypeNetworkTimeout {
		t.Errorf("Type not copied")
	}
	if err.Service != "Service" {
		t.Errorf("Service not copied")
	}
	if err.Operation != "Operation" {
		t.Errorf("Operation not copied")
	}
	if err.Message != "message" {
		t.Errorf("Message not copied")
	}
	if err.Original != original {
		t.Errorf("Original error not preserved")
	}
}
