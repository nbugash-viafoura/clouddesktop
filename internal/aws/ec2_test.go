package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// mockEC2API implements ec2api for testing.
type mockEC2API struct {
	startFn    func(ctx context.Context, params *ec2.StartInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StartInstancesOutput, error)
	stopFn     func(ctx context.Context, params *ec2.StopInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StopInstancesOutput, error)
	describeFn func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

func (m *mockEC2API) StartInstances(ctx context.Context, params *ec2.StartInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StartInstancesOutput, error) {
	if m.startFn != nil {
		return m.startFn(ctx, params, optFns...)
	}
	return &ec2.StartInstancesOutput{}, nil
}

func (m *mockEC2API) StopInstances(ctx context.Context, params *ec2.StopInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StopInstancesOutput, error) {
	if m.stopFn != nil {
		return m.stopFn(ctx, params, optFns...)
	}
	return &ec2.StopInstancesOutput{}, nil
}

func (m *mockEC2API) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	if m.describeFn != nil {
		return m.describeFn(ctx, params, optFns...)
	}
	return &ec2.DescribeInstancesOutput{}, nil
}

// fastClient creates an EC2Client with minimal polling intervals for fast tests.
func fastClient(api ec2api) *EC2Client {
	c := newEC2ClientWithAPI(api)
	c.pollInitialInterval = 1 * time.Millisecond
	c.pollMaxInterval = 5 * time.Millisecond
	return c
}

// Helper to build a DescribeInstances response with one instance in a given state.
func describeOutput(instanceID, state, privateIP string) *ec2.DescribeInstancesOutput {
	now := time.Now()
	return &ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId:       &instanceID,
						State:            &types.InstanceState{Name: types.InstanceStateName(state)},
						InstanceType:     types.InstanceTypeM7iXlarge,
						PrivateIpAddress: &privateIP,
						LaunchTime:       &now,
					},
				},
			},
		},
	}
}

func TestStartInstance_Success(t *testing.T) {
	mock := &mockEC2API{}
	client := newEC2ClientWithAPI(mock)

	err := client.StartInstance(context.Background(), "i-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartInstance_Error(t *testing.T) {
	mock := &mockEC2API{
		startFn: func(ctx context.Context, params *ec2.StartInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StartInstancesOutput, error) {
			return nil, errors.New("access denied")
		},
	}
	client := newEC2ClientWithAPI(mock)

	err := client.StartInstance(context.Background(), "i-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to start instance") {
		t.Errorf("error = %q, want to contain 'failed to start instance'", err)
	}
}

func TestStopInstance_Success(t *testing.T) {
	mock := &mockEC2API{}
	client := newEC2ClientWithAPI(mock)

	err := client.StopInstance(context.Background(), "i-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStopInstance_Error(t *testing.T) {
	mock := &mockEC2API{
		stopFn: func(ctx context.Context, params *ec2.StopInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StopInstancesOutput, error) {
			return nil, errors.New("access denied")
		},
	}
	client := newEC2ClientWithAPI(mock)

	err := client.StopInstance(context.Background(), "i-123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to stop instance") {
		t.Errorf("error = %q, want to contain 'failed to stop instance'", err)
	}
}

func TestDescribeInstance_Success(t *testing.T) {
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return describeOutput("i-123", "running", "10.0.1.5"), nil
		},
	}
	client := newEC2ClientWithAPI(mock)

	info, err := client.DescribeInstance(context.Background(), "i-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.InstanceID != "i-123" {
		t.Errorf("InstanceID = %q, want i-123", info.InstanceID)
	}
	if info.State != "running" {
		t.Errorf("State = %q, want running", info.State)
	}
	if info.PrivateIP != "10.0.1.5" {
		t.Errorf("PrivateIP = %q, want 10.0.1.5", info.PrivateIP)
	}
}

func TestDescribeInstance_NotFound(t *testing.T) {
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{Reservations: []types.Reservation{}}, nil
		},
	}
	client := newEC2ClientWithAPI(mock)

	_, err := client.DescribeInstance(context.Background(), "i-missing")
	if err == nil {
		t.Fatal("expected error for missing instance")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain 'not found'", err)
	}
}

func TestDescribeInstance_Error(t *testing.T) {
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return nil, errors.New("network failure")
		},
	}
	client := newEC2ClientWithAPI(mock)

	_, err := client.DescribeInstance(context.Background(), "i-123")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to describe instance") {
		t.Errorf("error = %q, want to contain 'failed to describe instance'", err)
	}
}

func TestDescribeInstance_NilPrivateIP(t *testing.T) {
	now := time.Now()
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{
								InstanceId:       strPtr("i-123"),
								State:            &types.InstanceState{Name: types.InstanceStateNamePending},
								InstanceType:     types.InstanceTypeM7iXlarge,
								PrivateIpAddress: nil,
								LaunchTime:       &now,
							},
						},
					},
				},
			}, nil
		},
	}
	client := newEC2ClientWithAPI(mock)

	info, err := client.DescribeInstance(context.Background(), "i-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.PrivateIP != "" {
		t.Errorf("PrivateIP = %q, want empty for nil", info.PrivateIP)
	}
}

func TestWaitForState_ImmediateSuccess(t *testing.T) {
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return describeOutput("i-123", "running", "10.0.1.5"), nil
		},
	}
	client := fastClient(mock)

	err := client.WaitUntilRunning(context.Background(), "i-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForState_EventualSuccess(t *testing.T) {
	callCount := 0
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			callCount++
			if callCount < 3 {
				return describeOutput("i-123", "pending", ""), nil
			}
			return describeOutput("i-123", "running", "10.0.1.5"), nil
		},
	}
	client := fastClient(mock)

	err := client.WaitUntilRunning(context.Background(), "i-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount < 3 {
		t.Errorf("expected at least 3 describe calls, got %d", callCount)
	}
}

func TestWaitForState_TransientErrorRecovery(t *testing.T) {
	callCount := 0
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			callCount++
			if callCount == 1 {
				return nil, fmt.Errorf("failed to describe instance i-123: %w", errors.New("Throttling: rate exceeded"))
			}
			return describeOutput("i-123", "stopped", ""), nil
		},
	}
	client := fastClient(mock)

	err := client.WaitUntilStopped(context.Background(), "i-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitForState_TooManyTransientErrors(t *testing.T) {
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return nil, fmt.Errorf("failed to describe instance i-123: %w", errors.New("Throttling: rate exceeded"))
		},
	}
	client := fastClient(mock)

	err := client.waitForState(context.Background(), "i-123", "running")
	if err == nil {
		t.Fatal("expected error after too many transient failures")
	}
	if !strings.Contains(err.Error(), "giving up") {
		t.Errorf("error = %q, want to contain 'giving up'", err)
	}
}

func TestWaitForState_PermanentError(t *testing.T) {
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return nil, errors.New("instance i-123 not found")
		},
	}
	client := fastClient(mock)

	err := client.waitForState(context.Background(), "i-123", "running")
	if err == nil {
		t.Fatal("expected immediate error for permanent failure")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain 'not found'", err)
	}
}

func TestWaitForState_ContextCancelled(t *testing.T) {
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return describeOutput("i-123", "pending", ""), nil
		},
	}
	client := fastClient(mock)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := client.waitForState(ctx, "i-123", "running")
	if err == nil {
		t.Fatal("expected context cancelled error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

// --- Tests for pure functions (carried over from previous) ---

func TestIsTransientError_Nil(t *testing.T) {
	if isTransientError(nil) {
		t.Error("nil error should not be transient")
	}
}

func TestIsTransientError_TransientPatterns(t *testing.T) {
	transient := []struct {
		name string
		err  error
	}{
		{"Throttling", errors.New("Throttling: Rate exceeded")},
		{"RequestLimitExceeded", errors.New("RequestLimitExceeded: too many requests")},
		{"InternalError", errors.New("InternalError: something broke")},
		{"ServiceUnavailable", errors.New("ServiceUnavailable: try again")},
		{"RequestTimeout", errors.New("RequestTimeout: deadline exceeded")},
		{"connection reset", errors.New("read tcp: connection reset by peer")},
		{"i/o timeout", errors.New("dial tcp: i/o timeout")},
		{"TLS handshake timeout", errors.New("net/http: TLS handshake timeout")},
		{"wrapped transient", fmt.Errorf("describe failed: %w", errors.New("Throttling: rate limit"))},
	}

	for _, tt := range transient {
		t.Run(tt.name, func(t *testing.T) {
			if !isTransientError(tt.err) {
				t.Errorf("expected %q to be transient", tt.err)
			}
		})
	}
}

func TestIsTransientError_PermanentErrors(t *testing.T) {
	permanent := []struct {
		name string
		err  error
	}{
		{"AccessDenied", errors.New("AccessDenied: not authorized")},
		{"InvalidParameterValue", errors.New("InvalidParameterValue: bad input")},
		{"instance not found", errors.New("instance i-123 not found")},
		{"generic error", errors.New("something went wrong")},
		{"UnauthorizedOperation", errors.New("UnauthorizedOperation: missing perms")},
	}

	for _, tt := range permanent {
		t.Run(tt.name, func(t *testing.T) {
			if isTransientError(tt.err) {
				t.Errorf("expected %q to NOT be transient", tt.err)
			}
		})
	}
}

func TestPollConstants(t *testing.T) {
	if pollInitialInterval <= 0 {
		t.Error("pollInitialInterval should be positive")
	}
	if pollMaxInterval <= pollInitialInterval {
		t.Error("pollMaxInterval should be greater than pollInitialInterval")
	}
	if pollMaxConsecutiveErrors < 1 {
		t.Error("pollMaxConsecutiveErrors should be at least 1")
	}
}

