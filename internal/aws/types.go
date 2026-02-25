package aws

import "fmt"

// ErrorType represents the category of an AWS error.
type ErrorType int

const (
	ErrorTypeUnknown ErrorType = iota
	ErrorTypeNotFound
	ErrorTypePermissionDenied
	ErrorTypeNetworkTimeout
	ErrorTypeRateLimit
	ErrorTypeInvalidInput
	ErrorTypeServiceUnavailable
)

// AWSError is a custom error type that wraps AWS SDK errors with context.
type AWSError struct {
	Type      ErrorType
	Service   string // e.g. "EC2", "STS", "CloudWatch"
	Operation string // e.g. "DescribeInstances", "AssumeRole"
	Resource  string // e.g. instance ID, parameter name (optional)
	Message   string
	Original  error // the underlying error
}

// Error implements the error interface.
func (e *AWSError) Error() string {
	if e.Resource != "" {
		return fmt.Sprintf("%s.%s(%s): %s", e.Service, e.Operation, e.Resource, e.Message)
	}
	return fmt.Sprintf("%s.%s: %s", e.Service, e.Operation, e.Message)
}

// Unwrap returns the original error for error wrapping compatibility.
func (e *AWSError) Unwrap() error {
	return e.Original
}

// Remediation returns a suggested fix for the error.
func (e *AWSError) Remediation() string {
	switch e.Type {
	case ErrorTypePermissionDenied:
		return fmt.Sprintf("Verify your AWS profile is 'test-developers' and you have %s permissions for %s", e.Operation, e.Service)
	case ErrorTypeNotFound:
		return fmt.Sprintf("%s '%s' not found in %s. It may have been deleted or doesn't exist yet.", e.Operation, e.Resource, e.Service)
	case ErrorTypeNetworkTimeout:
		return "Check your internet connection and AWS API availability. Retry in a few moments."
	case ErrorTypeRateLimit:
		return "AWS API rate limit exceeded. Wait a few seconds and retry."
	case ErrorTypeServiceUnavailable:
		return fmt.Sprintf("%s is temporarily unavailable. Try again in a few moments.", e.Service)
	default:
		return "Check AWS CloudTrail logs for more details."
	}
}

// NewError creates a new AWSError.
func NewError(errorType ErrorType, service, operation, message string, original error) *AWSError {
	return &AWSError{
		Type:      errorType,
		Service:   service,
		Operation: operation,
		Message:   message,
		Original:  original,
	}
}

// NewErrorWithResource creates a new AWSError with a resource identifier.
func NewErrorWithResource(errorType ErrorType, service, operation, resource, message string, original error) *AWSError {
	return &AWSError{
		Type:      errorType,
		Service:   service,
		Operation: operation,
		Resource:  resource,
		Message:   message,
		Original:  original,
	}
}

// Helper functions for common errors

// ErrInstanceNotFound creates an error for a missing EC2 instance.
func ErrInstanceNotFound(instanceID string) *AWSError {
	return NewErrorWithResource(
		ErrorTypeNotFound,
		"EC2",
		"DescribeInstances",
		instanceID,
		fmt.Sprintf("instance %s not found", instanceID),
		nil,
	)
}

// ErrPermissionDenied creates an error for an AWS permission issue.
func ErrPermissionDenied(service, operation string) *AWSError {
	return NewError(
		ErrorTypePermissionDenied,
		service,
		operation,
		fmt.Sprintf("permission denied calling %s.%s", service, operation),
		nil,
	)
}

// ErrNetworkTimeout creates an error for a network timeout.
func ErrNetworkTimeout(service, operation string) *AWSError {
	return NewError(
		ErrorTypeNetworkTimeout,
		service,
		operation,
		"request timed out",
		nil,
	)
}

// ErrSessionExpired creates an error for an expired AWS session.
func ErrSessionExpired() error {
	return fmt.Errorf("AWS session expired or invalid - run 'sts' to refresh your credentials")
}

// ErrSSHKeyNotFound creates an error for a missing SSH key file.
func ErrSSHKeyNotFound(path string) error {
	return fmt.Errorf("SSH public key not found at %s - generate one with: ssh-keygen -t ed25519 -f ~/.ssh/viafoura_dev", path)
}

// ErrAWSProfileNotFound creates an error for a missing AWS profile.
func ErrAWSProfileNotFound(profile string) error {
	return fmt.Errorf("AWS profile '%s' not found - configure it in ~/.aws/config or ~/.aws/credentials", profile)
}
