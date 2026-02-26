package testing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nbugash-viafoura/clouddesktop/internal/aws"
	"github.com/nbugash-viafoura/clouddesktop/internal/config"
)

// MockEC2Client is a test double for aws.EC2Client.
type MockEC2Client struct {
	StartInstanceFn       func(ctx context.Context, instanceID string) error
	StopInstanceFn        func(ctx context.Context, instanceID string) error
	DescribeInstanceFn    func(ctx context.Context, instanceID string) (*aws.InstanceInfo, error)
	WaitUntilRunningFn    func(ctx context.Context, instanceID string) error
	WaitUntilStoppedFn    func(ctx context.Context, instanceID string) error
	StartInstanceCalls    int
	StopInstanceCalls     int
	DescribeInstanceCalls int
}

// StartInstance calls the mock function.
func (m *MockEC2Client) StartInstance(ctx context.Context, instanceID string) error {
	m.StartInstanceCalls++
	if m.StartInstanceFn != nil {
		return m.StartInstanceFn(ctx, instanceID)
	}
	return nil
}

// StopInstance calls the mock function.
func (m *MockEC2Client) StopInstance(ctx context.Context, instanceID string) error {
	m.StopInstanceCalls++
	if m.StopInstanceFn != nil {
		return m.StopInstanceFn(ctx, instanceID)
	}
	return nil
}

// DescribeInstance calls the mock function.
func (m *MockEC2Client) DescribeInstance(ctx context.Context, instanceID string) (*aws.InstanceInfo, error) {
	m.DescribeInstanceCalls++
	if m.DescribeInstanceFn != nil {
		return m.DescribeInstanceFn(ctx, instanceID)
	}
	launchTime := time.Now()
	return &aws.InstanceInfo{
		InstanceID:   instanceID,
		State:        "running",
		PrivateIP:    "10.200.1.100",
		InstanceType: "m7i.xlarge",
		LaunchTime:   &launchTime,
	}, nil
}

// WaitUntilRunning calls the mock function.
func (m *MockEC2Client) WaitUntilRunning(ctx context.Context, instanceID string) error {
	if m.WaitUntilRunningFn != nil {
		return m.WaitUntilRunningFn(ctx, instanceID)
	}
	return nil
}

// WaitUntilStopped calls the mock function.
func (m *MockEC2Client) WaitUntilStopped(ctx context.Context, instanceID string) error {
	if m.WaitUntilStoppedFn != nil {
		return m.WaitUntilStoppedFn(ctx, instanceID)
	}
	return nil
}

// MockCloudWatchClient is a test double for aws.CloudWatchClient.
type MockCloudWatchClient struct {
	GetInstanceMetricsFn    func(ctx context.Context, instanceID string) (*aws.InstanceMetrics, error)
	GetInstanceMetricsCalls int
}

// GetInstanceMetrics calls the mock function.
func (m *MockCloudWatchClient) GetInstanceMetrics(ctx context.Context, instanceID string) (*aws.InstanceMetrics, error) {
	m.GetInstanceMetricsCalls++
	if m.GetInstanceMetricsFn != nil {
		return m.GetInstanceMetricsFn(ctx, instanceID)
	}
	return &aws.InstanceMetrics{
		CPUPercent:    50.0,
		MemoryPercent: -1,
		DiskPercent:   -1,
	}, nil
}

// TestConfig returns a valid config for testing.
func TestConfig() *config.Config {
	return &config.Config{
		AWSProfile:      "test-developers",
		Region:          "us-east-1",
		InstanceType:    "m7i.xlarge",
		SSHPublicKey:    "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKlh4pZYi3EZg7OMPKDa1Nt2yw9Z8CZSmjQ2qC8i1nC8 test@example.com",
		SSHKeyPath:    "/home/test/.ssh/viafoura_dev",
		DeveloperName: "john-dev",
		InstanceID:      "i-0123456789abcdef0",
	}
}

// ValidConfig returns True if the config is valid.
func ValidConfig() *config.Config {
	return TestConfig()
}

// InvalidConfig returns a config with missing required fields.
func InvalidConfig() *config.Config {
	return &config.Config{
		AWSProfile:   "test-developers",
		Region:       "", // missing region
		InstanceType: "m7i.xlarge",
		DeveloperName: "john-dev",
	}
}

// AssertEqual checks if two values are equal, returning an error if not.
func AssertEqual(t interface{ Errorf(string, ...interface{}) }, expected, actual interface{}, message string) {
	if expected != actual {
		t.Errorf("%s: expected %v, got %v", message, expected, actual)
	}
}

// AssertError checks if an error occurred when one was expected.
func AssertError(t interface{ Errorf(string, ...interface{}) }, err error, message string) {
	if err == nil {
		t.Errorf("%s: expected error, got nil", message)
	}
}

// AssertNoError checks if an error did not occur when none was expected.
func AssertNoError(t interface{ Errorf(string, ...interface{}) }, err error, message string) {
	if err != nil {
		t.Errorf("%s: unexpected error: %v", message, err)
	}
}

// AssertErrorMessage checks if an error message contains a substring.
func AssertErrorMessage(t interface{ Errorf(string, ...interface{}) }, err error, substring, message string) {
	if err == nil {
		t.Errorf("%s: expected error, got nil", message)
		return
	}
	if !strings.Contains(err.Error(), substring) {
		t.Errorf("%s: expected error containing %q, got %q", message, substring, err.Error())
	}
}

// TempDir creates a temporary directory for testing (caller responsible for cleanup).
// This is a placeholder - tests should use t.TempDir() in Go 1.15+
type TempDirFunc func() (string, error)

// MockInstanceInfo creates a mock EC2 instance info.
func MockInstanceInfo(instanceID, state string) *aws.InstanceInfo {
	launchTime := time.Now()
	return &aws.InstanceInfo{
		InstanceID:   instanceID,
		State:        state,
		PrivateIP:    "10.200.1.100",
		InstanceType: "m7i.xlarge",
		LaunchTime:   &launchTime,
	}
}

// MockAWSError creates a mock AWS error for testing error handling.
func MockAWSError(errorType aws.ErrorType, message string) *aws.AWSError {
	return aws.NewError(errorType, "EC2", "TestOperation", message, fmt.Errorf("original error"))
}
