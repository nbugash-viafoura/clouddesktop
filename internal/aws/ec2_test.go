package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// mockEC2API implements ec2api for testing.
type mockEC2API struct {
	startFn       func(ctx context.Context, params *ec2.StartInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StartInstancesOutput, error)
	stopFn        func(ctx context.Context, params *ec2.StopInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StopInstancesOutput, error)
	describeFn    func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	runFn         func(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error)
	terminateFn   func(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
	importKeyFn   func(ctx context.Context, params *ec2.ImportKeyPairInput, optFns ...func(*ec2.Options)) (*ec2.ImportKeyPairOutput, error)
	deleteKeyFn   func(ctx context.Context, params *ec2.DeleteKeyPairInput, optFns ...func(*ec2.Options)) (*ec2.DeleteKeyPairOutput, error)
	describeImgFn func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
	modifyAttrFn  func(ctx context.Context, params *ec2.ModifyInstanceAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyInstanceAttributeOutput, error)
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

func (m *mockEC2API) RunInstances(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
	if m.runFn != nil {
		return m.runFn(ctx, params, optFns...)
	}
	return &ec2.RunInstancesOutput{}, nil
}

func (m *mockEC2API) TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {
	if m.terminateFn != nil {
		return m.terminateFn(ctx, params, optFns...)
	}
	return &ec2.TerminateInstancesOutput{}, nil
}

func (m *mockEC2API) ImportKeyPair(ctx context.Context, params *ec2.ImportKeyPairInput, optFns ...func(*ec2.Options)) (*ec2.ImportKeyPairOutput, error) {
	if m.importKeyFn != nil {
		return m.importKeyFn(ctx, params, optFns...)
	}
	return &ec2.ImportKeyPairOutput{}, nil
}

func (m *mockEC2API) DeleteKeyPair(ctx context.Context, params *ec2.DeleteKeyPairInput, optFns ...func(*ec2.Options)) (*ec2.DeleteKeyPairOutput, error) {
	if m.deleteKeyFn != nil {
		return m.deleteKeyFn(ctx, params, optFns...)
	}
	return &ec2.DeleteKeyPairOutput{}, nil
}

func (m *mockEC2API) DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	if m.describeImgFn != nil {
		return m.describeImgFn(ctx, params, optFns...)
	}
	return &ec2.DescribeImagesOutput{}, nil
}

func (m *mockEC2API) ModifyInstanceAttribute(ctx context.Context, params *ec2.ModifyInstanceAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyInstanceAttributeOutput, error) {
	if m.modifyAttrFn != nil {
		return m.modifyAttrFn(ctx, params, optFns...)
	}
	return &ec2.ModifyInstanceAttributeOutput{}, nil
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
	if !strings.Contains(err.Error(), "Throttling") && !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain 'Throttling' or 'not found'", err)
	}
}

func TestWaitForState_PermanentError(t *testing.T) {
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return nil, errors.New("AccessDeniedException: not authorized")
		},
	}
	client := fastClient(mock)

	err := client.waitForState(context.Background(), "i-123", "running")
	if err == nil {
		t.Fatal("expected immediate error for permanent failure")
	}
	if !strings.Contains(err.Error(), "not authorized") {
		t.Errorf("error = %q, want to contain 'not authorized'", err)
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

// --- Tests for new provisioning methods ---

func TestFindInstanceByDeveloper_Found(t *testing.T) {
	now := time.Now()
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{
								InstanceId:       aws.String("i-existing"),
								State:            &types.InstanceState{Name: types.InstanceStateNameRunning},
								InstanceType:     types.InstanceTypeM7iXlarge,
								PrivateIpAddress: aws.String("10.0.1.50"),
								LaunchTime:       &now,
							},
						},
					},
				},
			}, nil
		},
	}
	client := newEC2ClientWithAPI(mock)

	info, err := client.FindInstanceByDeveloper(context.Background(), "john-dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("expected to find instance, got nil")
	}
	if info.InstanceID != "i-existing" {
		t.Errorf("InstanceID = %q, want i-existing", info.InstanceID)
	}
	if info.State != "running" {
		t.Errorf("State = %q, want running", info.State)
	}
}

func TestFindInstanceByDeveloper_NotFound(t *testing.T) {
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{Reservations: []types.Reservation{}}, nil
		},
	}
	client := newEC2ClientWithAPI(mock)

	info, err := client.FindInstanceByDeveloper(context.Background(), "john-dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info != nil {
		t.Errorf("expected nil, got %+v", info)
	}
}

func TestFindInstanceByDeveloper_APIError(t *testing.T) {
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return nil, errors.New("access denied")
		},
	}
	client := newEC2ClientWithAPI(mock)

	_, err := client.FindInstanceByDeveloper(context.Background(), "john-dev")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTerminateInstance_Success(t *testing.T) {
	mock := &mockEC2API{}
	client := newEC2ClientWithAPI(mock)

	err := client.TerminateInstance(context.Background(), "i-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTerminateInstance_Error(t *testing.T) {
	mock := &mockEC2API{
		terminateFn: func(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {
			return nil, errors.New("access denied")
		},
	}
	client := newEC2ClientWithAPI(mock)

	err := client.TerminateInstance(context.Background(), "i-123")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to terminate instance") {
		t.Errorf("error = %q, want to contain 'failed to terminate instance'", err)
	}
}

func TestImportSSHKeyPair_Success(t *testing.T) {
	mock := &mockEC2API{}
	client := newEC2ClientWithAPI(mock)

	err := client.ImportSSHKeyPair(context.Background(), "clouddesktop-john", "ssh-ed25519 AAAA...")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImportSSHKeyPair_Error(t *testing.T) {
	mock := &mockEC2API{
		importKeyFn: func(ctx context.Context, params *ec2.ImportKeyPairInput, optFns ...func(*ec2.Options)) (*ec2.ImportKeyPairOutput, error) {
			return nil, errors.New("invalid key")
		},
	}
	client := newEC2ClientWithAPI(mock)

	err := client.ImportSSHKeyPair(context.Background(), "clouddesktop-john", "bad-key")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to import key pair") {
		t.Errorf("error = %q, want to contain 'failed to import key pair'", err)
	}
}

func TestModifyInstanceType_Success(t *testing.T) {
	mock := &mockEC2API{}
	client := newEC2ClientWithAPI(mock)

	err := client.ModifyInstanceType(context.Background(), "i-123", "m7i.2xlarge")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestModifyInstanceType_Error(t *testing.T) {
	mock := &mockEC2API{
		modifyAttrFn: func(ctx context.Context, params *ec2.ModifyInstanceAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyInstanceAttributeOutput, error) {
			return nil, errors.New("instance not stopped")
		},
	}
	client := newEC2ClientWithAPI(mock)

	err := client.ModifyInstanceType(context.Background(), "i-123", "m7i.2xlarge")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to modify instance type") {
		t.Errorf("error = %q, want to contain 'failed to modify instance type'", err)
	}
}

func TestFindUbuntuAMI_Success(t *testing.T) {
	mock := &mockEC2API{
		describeImgFn: func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{
				Images: []types.Image{
					{ImageId: aws.String("ami-older"), CreationDate: aws.String("2024-01-01T00:00:00Z")},
					{ImageId: aws.String("ami-latest"), CreationDate: aws.String("2024-06-01T00:00:00Z")},
					{ImageId: aws.String("ami-middle"), CreationDate: aws.String("2024-03-01T00:00:00Z")},
				},
			}, nil
		},
	}
	client := newEC2ClientWithAPI(mock)

	amiID, err := client.FindUbuntuAMI(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if amiID != "ami-latest" {
		t.Errorf("AMI ID = %q, want ami-latest", amiID)
	}
}

func TestFindUbuntuAMI_NoImages(t *testing.T) {
	mock := &mockEC2API{
		describeImgFn: func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{Images: []types.Image{}}, nil
		},
	}
	client := newEC2ClientWithAPI(mock)

	_, err := client.FindUbuntuAMI(context.Background())
	if err == nil {
		t.Fatal("expected error for no AMIs")
	}
	if !strings.Contains(err.Error(), "no Ubuntu 22.04 AMI found") {
		t.Errorf("error = %q, want to contain 'no Ubuntu 22.04 AMI found'", err)
	}
}

func TestRunInstance_Success(t *testing.T) {
	mock := &mockEC2API{
		runFn: func(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
			return &ec2.RunInstancesOutput{
				Instances: []types.Instance{
					{InstanceId: aws.String("i-new123")},
				},
			}, nil
		},
	}
	client := newEC2ClientWithAPI(mock)

	instanceID, err := client.RunInstance(context.Background(), RunInstanceParams{
		AMIID:               "ami-123",
		InstanceType:        "m7i.xlarge",
		SubnetID:            "subnet-abc",
		SecurityGroupID:     "sg-def",
		KeyName:             "clouddesktop-john",
		InstanceProfileName: "clouddesktop-developer-instance",
		UserData:            []byte("#!/bin/bash\necho hello"),
		DeveloperName:       "john-dev",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if instanceID != "i-new123" {
		t.Errorf("instanceID = %q, want i-new123", instanceID)
	}
}

func TestRunInstance_Error(t *testing.T) {
	mock := &mockEC2API{
		runFn: func(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
			return nil, errors.New("insufficient capacity")
		},
	}
	client := newEC2ClientWithAPI(mock)

	_, err := client.RunInstance(context.Background(), RunInstanceParams{
		AMIID:         "ami-123",
		InstanceType:  "m7i.xlarge",
		DeveloperName: "john-dev",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to launch instance") {
		t.Errorf("error = %q, want to contain 'failed to launch instance'", err)
	}
}

