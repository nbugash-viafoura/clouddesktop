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
	startFn              func(ctx context.Context, params *ec2.StartInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StartInstancesOutput, error)
	stopFn               func(ctx context.Context, params *ec2.StopInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StopInstancesOutput, error)
	describeFn           func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	runFn                func(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error)
	terminateFn          func(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
	importKeyFn          func(ctx context.Context, params *ec2.ImportKeyPairInput, optFns ...func(*ec2.Options)) (*ec2.ImportKeyPairOutput, error)
	deleteKeyFn          func(ctx context.Context, params *ec2.DeleteKeyPairInput, optFns ...func(*ec2.Options)) (*ec2.DeleteKeyPairOutput, error)
	describeImgFn        func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
	modifyAttrFn         func(ctx context.Context, params *ec2.ModifyInstanceAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyInstanceAttributeOutput, error)
	describeVolumesFn    func(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
	modifyVolumeFn       func(ctx context.Context, params *ec2.ModifyVolumeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyVolumeOutput, error)
	describeVolumeModsFn func(ctx context.Context, params *ec2.DescribeVolumesModificationsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesModificationsOutput, error)
	createTagsFn         func(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)
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

func (m *mockEC2API) DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	if m.describeVolumesFn != nil {
		return m.describeVolumesFn(ctx, params, optFns...)
	}
	return &ec2.DescribeVolumesOutput{}, nil
}

func (m *mockEC2API) ModifyVolume(ctx context.Context, params *ec2.ModifyVolumeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyVolumeOutput, error) {
	if m.modifyVolumeFn != nil {
		return m.modifyVolumeFn(ctx, params, optFns...)
	}
	return &ec2.ModifyVolumeOutput{}, nil
}

func (m *mockEC2API) DescribeVolumesModifications(ctx context.Context, params *ec2.DescribeVolumesModificationsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesModificationsOutput, error) {
	if m.describeVolumeModsFn != nil {
		return m.describeVolumeModsFn(ctx, params, optFns...)
	}
	return &ec2.DescribeVolumesModificationsOutput{}, nil
}

func (m *mockEC2API) CreateTags(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
	if m.createTagsFn != nil {
		return m.createTagsFn(ctx, params, optFns...)
	}
	return &ec2.CreateTagsOutput{}, nil
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
	if !strings.Contains(err.Error(), "no Ubuntu 24.04 AMI found") {
		t.Errorf("error = %q, want to contain 'no Ubuntu 24.04 AMI found'", err)
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

// --- Tests for EBS volume methods ---

// describeInstanceWithVolume builds a DescribeInstances response with block device mapping.
func describeInstanceWithVolume(instanceID, rootDevice, volumeID string) *ec2.DescribeInstancesOutput {
	now := time.Now()
	return &ec2.DescribeInstancesOutput{
		Reservations: []types.Reservation{
			{
				Instances: []types.Instance{
					{
						InstanceId:       aws.String(instanceID),
						State:            &types.InstanceState{Name: types.InstanceStateNameRunning},
						InstanceType:     types.InstanceTypeM7iXlarge,
						PrivateIpAddress: aws.String("10.0.1.5"),
						LaunchTime:       &now,
						RootDeviceName:   aws.String(rootDevice),
						BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
							{
								DeviceName: aws.String(rootDevice),
								Ebs: &types.EbsInstanceBlockDevice{
									VolumeId: aws.String(volumeID),
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestGetRootVolumeInfo_Success(t *testing.T) {
	var size int32 = 100
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return describeInstanceWithVolume("i-123", "/dev/sda1", "vol-abc"), nil
		},
		describeVolumesFn: func(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
			return &ec2.DescribeVolumesOutput{
				Volumes: []types.Volume{{VolumeId: aws.String("vol-abc"), Size: &size}},
			}, nil
		},
	}
	client := newEC2ClientWithAPI(mock)

	volumeID, sizeGB, err := client.GetRootVolumeInfo(context.Background(), "i-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if volumeID != "vol-abc" {
		t.Errorf("volumeID = %q, want vol-abc", volumeID)
	}
	if sizeGB != 100 {
		t.Errorf("sizeGB = %d, want 100", sizeGB)
	}
}

func TestGetRootVolumeInfo_InstanceNotFound(t *testing.T) {
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{Reservations: []types.Reservation{}}, nil
		},
	}
	client := newEC2ClientWithAPI(mock)

	_, _, err := client.GetRootVolumeInfo(context.Background(), "i-missing")
	if err == nil {
		t.Fatal("expected error for missing instance")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain 'not found'", err)
	}
}

func TestGetRootVolumeInfo_NoRootDevice(t *testing.T) {
	now := time.Now()
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{
								InstanceId:          aws.String("i-123"),
								State:               &types.InstanceState{Name: types.InstanceStateNameRunning},
								RootDeviceName:      nil, // no root device
								BlockDeviceMappings: []types.InstanceBlockDeviceMapping{},
								LaunchTime:          &now,
							},
						},
					},
				},
			}, nil
		},
	}
	client := newEC2ClientWithAPI(mock)

	_, _, err := client.GetRootVolumeInfo(context.Background(), "i-123")
	if err == nil {
		t.Fatal("expected error for missing root device name")
	}
	if !strings.Contains(err.Error(), "no root device name") {
		t.Errorf("error = %q, want to contain 'no root device name'", err)
	}
}

func TestModifyRootVolume_Success(t *testing.T) {
	mock := &mockEC2API{}
	client := newEC2ClientWithAPI(mock)

	err := client.ModifyRootVolume(context.Background(), "vol-abc", 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestModifyRootVolume_Error(t *testing.T) {
	mock := &mockEC2API{
		modifyVolumeFn: func(ctx context.Context, params *ec2.ModifyVolumeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyVolumeOutput, error) {
			return nil, errors.New("size must be larger than current size")
		},
	}
	client := newEC2ClientWithAPI(mock)

	err := client.ModifyRootVolume(context.Background(), "vol-abc", 50)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to modify volume") {
		t.Errorf("error = %q, want to contain 'failed to modify volume'", err)
	}
}

func TestWaitUntilVolumeResized_ImmediateOptimizing(t *testing.T) {
	mock := &mockEC2API{
		describeVolumeModsFn: func(ctx context.Context, params *ec2.DescribeVolumesModificationsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesModificationsOutput, error) {
			return &ec2.DescribeVolumesModificationsOutput{
				VolumesModifications: []types.VolumeModification{
					{ModificationState: types.VolumeModificationStateOptimizing},
				},
			}, nil
		},
	}
	client := fastClient(mock)

	err := client.WaitUntilVolumeResized(context.Background(), "vol-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWaitUntilVolumeResized_EventuallyCompleted(t *testing.T) {
	callCount := 0
	mock := &mockEC2API{
		describeVolumeModsFn: func(ctx context.Context, params *ec2.DescribeVolumesModificationsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesModificationsOutput, error) {
			callCount++
			state := types.VolumeModificationStateModifying
			if callCount >= 3 {
				state = types.VolumeModificationStateCompleted
			}
			return &ec2.DescribeVolumesModificationsOutput{
				VolumesModifications: []types.VolumeModification{
					{ModificationState: state},
				},
			}, nil
		},
	}
	client := fastClient(mock)

	err := client.WaitUntilVolumeResized(context.Background(), "vol-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount < 3 {
		t.Errorf("expected at least 3 describe calls, got %d", callCount)
	}
}

func TestWaitUntilVolumeResized_Failed(t *testing.T) {
	failMsg := "volume modification request failed"
	mock := &mockEC2API{
		describeVolumeModsFn: func(ctx context.Context, params *ec2.DescribeVolumesModificationsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesModificationsOutput, error) {
			return &ec2.DescribeVolumesModificationsOutput{
				VolumesModifications: []types.VolumeModification{
					{
						ModificationState: types.VolumeModificationStateFailed,
						StatusMessage:     aws.String(failMsg),
					},
				},
			}, nil
		},
	}
	client := fastClient(mock)

	err := client.WaitUntilVolumeResized(context.Background(), "vol-abc")
	if err == nil {
		t.Fatal("expected error for failed modification")
	}
	if !strings.Contains(err.Error(), failMsg) {
		t.Errorf("error = %q, want to contain %q", err, failMsg)
	}
}

func TestWaitUntilVolumeResized_ContextCancelled(t *testing.T) {
	mock := &mockEC2API{
		describeVolumeModsFn: func(ctx context.Context, params *ec2.DescribeVolumesModificationsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesModificationsOutput, error) {
			return &ec2.DescribeVolumesModificationsOutput{
				VolumesModifications: []types.VolumeModification{
					{ModificationState: types.VolumeModificationStateModifying},
				},
			}, nil
		},
	}
	client := fastClient(mock)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.WaitUntilVolumeResized(ctx, "vol-abc")
	if err == nil {
		t.Fatal("expected context cancelled error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestWaitUntilVolumeResized_TransientErrorRetried(t *testing.T) {
	callCount := 0
	mock := &mockEC2API{
		describeVolumeModsFn: func(ctx context.Context, params *ec2.DescribeVolumesModificationsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesModificationsOutput, error) {
			callCount++
			if callCount == 1 {
				return nil, fmt.Errorf("failed to describe volume modifications: %w", errors.New("Throttling: rate exceeded"))
			}
			return &ec2.DescribeVolumesModificationsOutput{
				VolumesModifications: []types.VolumeModification{
					{ModificationState: types.VolumeModificationStateOptimizing},
				},
			}, nil
		},
	}
	client := fastClient(mock)

	err := client.WaitUntilVolumeResized(context.Background(), "vol-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 describe calls (retry after transient error), got %d", callCount)
	}
}

// --- Tests for environment listing and tagging ---

// envInstance builds an instance tagged with the given environment name.
// An empty envName leaves the CloudDesktopEnv tag off, like a legacy instance.
func envInstance(instanceID, envName, state string) types.Instance {
	tags := []types.Tag{
		{Key: aws.String("Developer"), Value: aws.String("john-dev")},
		{Key: aws.String("Project"), Value: aws.String("clouddesktop")},
	}
	if envName != "" {
		tags = append(tags, types.Tag{Key: aws.String("CloudDesktopEnv"), Value: aws.String(envName)})
	}
	now := time.Now()
	return types.Instance{
		InstanceId:       aws.String(instanceID),
		State:            &types.InstanceState{Name: types.InstanceStateName(state)},
		InstanceType:     types.InstanceTypeM7iXlarge,
		PrivateIpAddress: aws.String("10.0.1.5"),
		LaunchTime:       &now,
		Tags:             tags,
		RootDeviceName:   aws.String("/dev/sda1"),
		BlockDeviceMappings: []types.InstanceBlockDeviceMapping{
			{DeviceName: aws.String("/dev/sdf"), Ebs: &types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-data-" + instanceID)}},
			{DeviceName: aws.String("/dev/sda1"), Ebs: &types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-root-" + instanceID)}},
		},
	}
}

func filterValues(filters []types.Filter, name string) []string {
	for _, f := range filters {
		if aws.ToString(f.Name) == name {
			return f.Values
		}
	}
	return nil
}

func TestListEnvironments_FiltersAndMapping(t *testing.T) {
	var gotFilters []types.Filter
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			gotFilters = params.Filters
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{Instances: []types.Instance{envInstance("i-service", "service", "stopped")}},
					{Instances: []types.Instance{envInstance("i-build", "build", "running")}},
				},
			}, nil
		},
	}
	client := newEC2ClientWithAPI(mock)

	envs, err := client.ListEnvironments(context.Background(), "john-dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := filterValues(gotFilters, "tag:Developer"); len(got) != 1 || got[0] != "john-dev" {
		t.Errorf("tag:Developer filter = %v, want [john-dev]", got)
	}
	if got := filterValues(gotFilters, "tag:Project"); len(got) != 1 || got[0] != "clouddesktop" {
		t.Errorf("tag:Project filter = %v, want [clouddesktop]", got)
	}
	wantStates := []string{"pending", "running", "stopping", "stopped"}
	if got := filterValues(gotFilters, "instance-state-name"); strings.Join(got, ",") != strings.Join(wantStates, ",") {
		t.Errorf("instance-state-name filter = %v, want %v", got, wantStates)
	}

	if len(envs) != 2 {
		t.Fatalf("got %d environments, want 2", len(envs))
	}
	if envs[0].Name != "build" || envs[1].Name != "service" {
		t.Errorf("environments not sorted by name: %q, %q", envs[0].Name, envs[1].Name)
	}
	build := envs[0]
	if build.InstanceID != "i-build" || build.State != "running" || build.InstanceType != "m7i.xlarge" {
		t.Errorf("unexpected mapping: %+v", build)
	}
	if build.RootVolumeID != "vol-root-i-build" {
		t.Errorf("RootVolumeID = %q, want vol-root-i-build", build.RootVolumeID)
	}
	if build.PrivateIP != "10.0.1.5" || build.LaunchTime == nil {
		t.Errorf("PrivateIP/LaunchTime not mapped: %+v", build)
	}
	if build.Legacy {
		t.Error("tagged instance should not be marked legacy")
	}
}

func TestListEnvironments_LegacyInstanceIsMain(t *testing.T) {
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{{Instances: []types.Instance{envInstance("i-legacy", "", "running")}}},
			}, nil
		},
	}
	client := newEC2ClientWithAPI(mock)

	envs, err := client.ListEnvironments(context.Background(), "john-dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(envs) != 1 || envs[0].Name != "main" || !envs[0].Legacy {
		t.Errorf("got %+v, want one legacy environment named main", envs)
	}
}

func TestListEnvironments_FollowsPages(t *testing.T) {
	calls := 0
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			calls++
			if params.NextToken == nil {
				return &ec2.DescribeInstancesOutput{
					Reservations: []types.Reservation{{Instances: []types.Instance{envInstance("i-a", "alpha", "running")}}},
					NextToken:    aws.String("page-2"),
				}, nil
			}
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{{Instances: []types.Instance{envInstance("i-b", "beta", "stopped")}}},
			}, nil
		},
	}
	client := newEC2ClientWithAPI(mock)

	envs, err := client.ListEnvironments(context.Background(), "john-dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Errorf("DescribeInstances calls = %d, want 2", calls)
	}
	if len(envs) != 2 {
		t.Errorf("got %d environments, want 2", len(envs))
	}
}

func TestListEnvironments_DuplicateNameIsAnError(t *testing.T) {
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{Instances: []types.Instance{envInstance("i-one", "build", "running")}},
					{Instances: []types.Instance{envInstance("i-two", "build", "stopped")}},
				},
			}, nil
		},
	}
	client := newEC2ClientWithAPI(mock)

	_, err := client.ListEnvironments(context.Background(), "john-dev")
	if err == nil {
		t.Fatal("expected an error for two instances with the same environment name")
	}
	for _, want := range []string{`"build"`, "i-one", "i-two"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want to contain %s", err, want)
		}
	}
}

func TestListEnvironments_APIError(t *testing.T) {
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return nil, errors.New("access denied")
		},
	}
	client := newEC2ClientWithAPI(mock)

	if _, err := client.ListEnvironments(context.Background(), "john-dev"); err == nil {
		t.Fatal("expected error")
	}
}

func TestTagEnvironment(t *testing.T) {
	var got *ec2.CreateTagsInput
	mock := &mockEC2API{
		createTagsFn: func(ctx context.Context, params *ec2.CreateTagsInput, optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error) {
			got = params
			return &ec2.CreateTagsOutput{}, nil
		},
	}
	client := newEC2ClientWithAPI(mock)

	if err := client.TagEnvironment(context.Background(), "i-legacy", "vol-root", "main"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Join(got.Resources, ",") != "i-legacy,vol-root" {
		t.Errorf("Resources = %v, want [i-legacy vol-root]", got.Resources)
	}
	if len(got.Tags) != 1 || aws.ToString(got.Tags[0].Key) != "CloudDesktopEnv" || aws.ToString(got.Tags[0].Value) != "main" {
		t.Errorf("Tags = %+v, want CloudDesktopEnv=main", got.Tags)
	}
}

func tagMap(tags []types.Tag) map[string]string {
	m := make(map[string]string, len(tags))
	for _, tag := range tags {
		m[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	return m
}

func TestRunInstance_EnvironmentTagsAndVolumeSize(t *testing.T) {
	var got *ec2.RunInstancesInput
	mock := &mockEC2API{
		runFn: func(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
			got = params
			return &ec2.RunInstancesOutput{Instances: []types.Instance{{InstanceId: aws.String("i-new")}}}, nil
		},
	}
	client := newEC2ClientWithAPI(mock)

	_, err := client.RunInstance(context.Background(), RunInstanceParams{
		AMIID:            "ami-123",
		InstanceType:     "m7i.large",
		DeveloperName:    "john-dev",
		EnvironmentName:  "build",
		RootVolumeSizeGB: 300,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var instanceTags, volumeTags map[string]string
	for _, spec := range got.TagSpecifications {
		switch spec.ResourceType {
		case types.ResourceTypeInstance:
			instanceTags = tagMap(spec.Tags)
		case types.ResourceTypeVolume:
			volumeTags = tagMap(spec.Tags)
		}
	}
	if instanceTags["Name"] != "clouddesktop-john-dev-build" || instanceTags["CloudDesktopEnv"] != "build" {
		t.Errorf("instance tags = %v", instanceTags)
	}
	if volumeTags["Name"] != "clouddesktop-john-dev-build-root" || volumeTags["CloudDesktopEnv"] != "build" {
		t.Errorf("volume tags = %v", volumeTags)
	}
	if size := aws.ToInt32(got.BlockDeviceMappings[0].Ebs.VolumeSize); size != 300 {
		t.Errorf("VolumeSize = %d, want 300", size)
	}
}

func TestRunInstance_DefaultVolumeSize(t *testing.T) {
	var got *ec2.RunInstancesInput
	mock := &mockEC2API{
		runFn: func(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
			got = params
			return &ec2.RunInstancesOutput{Instances: []types.Instance{{InstanceId: aws.String("i-new")}}}, nil
		},
	}
	client := newEC2ClientWithAPI(mock)

	if _, err := client.RunInstance(context.Background(), RunInstanceParams{DeveloperName: "john-dev", EnvironmentName: "main"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size := aws.ToInt32(got.BlockDeviceMappings[0].Ebs.VolumeSize); size != 100 {
		t.Errorf("VolumeSize = %d, want default 100", size)
	}
}

func TestImportSSHKeyPair_DuplicateIsSuccess(t *testing.T) {
	mock := &mockEC2API{
		importKeyFn: func(ctx context.Context, params *ec2.ImportKeyPairInput, optFns ...func(*ec2.Options)) (*ec2.ImportKeyPairOutput, error) {
			return nil, errors.New("api error InvalidKeyPair.Duplicate: The keypair already exists")
		},
	}
	client := newEC2ClientWithAPI(mock)

	if err := client.ImportSSHKeyPair(context.Background(), "clouddesktop-john", "ssh-ed25519 AAAA..."); err != nil {
		t.Errorf("unexpected error for duplicate key pair: %v", err)
	}
}

func TestCapacityErrors(t *testing.T) {
	tests := []struct {
		name     string
		apiError string
		want     []string
	}{
		{
			name:     "vCPU quota",
			apiError: "api error VcpuLimitExceeded: You have requested more vCPU capacity than your current vCPU limit",
			want:     []string{"on-demand instance limit has been reached", "Stop environments you aren't using"},
		},
		{
			name:     "instance quota",
			apiError: "api error InstanceLimitExceeded: Your quota allows for 0 more running instance(s)",
			want:     []string{"on-demand instance limit has been reached", "ask the platform team"},
		},
		{
			name:     "no AWS capacity",
			apiError: "api error InsufficientInstanceCapacity: We currently do not have sufficient capacity",
			want:     []string{"AWS has no spare capacity for m7i.4xlarge right now", "choose another instance type"},
		},
	}

	for _, tt := range tests {
		t.Run("RunInstance "+tt.name, func(t *testing.T) {
			mock := &mockEC2API{
				runFn: func(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
					return nil, errors.New(tt.apiError)
				},
			}
			client := newEC2ClientWithAPI(mock)

			_, err := client.RunInstance(context.Background(), RunInstanceParams{InstanceType: "m7i.4xlarge", DeveloperName: "john-dev", EnvironmentName: "build"})

			var awsErr *AWSError
			if !errors.As(err, &awsErr) || awsErr.Type != ErrorTypeCapacity {
				t.Fatalf("error = %v, want *AWSError with ErrorTypeCapacity", err)
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error = %q, want to contain %q", err, want)
				}
			}
		})
	}

	t.Run("StartInstance quota", func(t *testing.T) {
		mock := &mockEC2API{
			startFn: func(ctx context.Context, params *ec2.StartInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StartInstancesOutput, error) {
				return nil, errors.New("api error VcpuLimitExceeded: limit")
			},
		}
		client := newEC2ClientWithAPI(mock)

		err := client.StartInstance(context.Background(), "i-123")

		var awsErr *AWSError
		if !errors.As(err, &awsErr) || awsErr.Type != ErrorTypeCapacity {
			t.Fatalf("error = %v, want *AWSError with ErrorTypeCapacity", err)
		}
	})
}

func TestRootVolumeSizes(t *testing.T) {
	calls := 0
	mock := &mockEC2API{
		describeVolumesFn: func(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
			calls++
			if strings.Join(params.VolumeIds, ",") != "vol-a,vol-b" {
				t.Errorf("VolumeIds = %v, want [vol-a vol-b]", params.VolumeIds)
			}
			return &ec2.DescribeVolumesOutput{Volumes: []types.Volume{
				{VolumeId: aws.String("vol-a"), Size: aws.Int32(100)},
				{VolumeId: aws.String("vol-b"), Size: aws.Int32(300)},
			}}, nil
		},
	}
	client := newEC2ClientWithAPI(mock)

	sizes, err := client.RootVolumeSizes(context.Background(), []string{"vol-a", "vol-b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 || sizes["vol-a"] != 100 || sizes["vol-b"] != 300 {
		t.Errorf("calls=%d sizes=%v, want 1 call with vol-a=100 vol-b=300", calls, sizes)
	}
}

func TestRootVolumeSizes_EmptyInputMakesNoCall(t *testing.T) {
	mock := &mockEC2API{
		describeVolumesFn: func(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
			t.Error("DescribeVolumes should not be called for no volumes")
			return &ec2.DescribeVolumesOutput{}, nil
		},
	}

	sizes, err := newEC2ClientWithAPI(mock).RootVolumeSizes(context.Background(), nil)
	if err != nil || len(sizes) != 0 {
		t.Errorf("got (%v, %v), want an empty map and no error", sizes, err)
	}
}

// SC-004: listing cost depends on result pages, not on the number of environments.
func TestListingTenEnvironments_UsesConstantCalls(t *testing.T) {
	describeCalls, volumeCalls := 0, 0
	mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			describeCalls++
			first := 0
			var next *string
			if params.NextToken == nil {
				next = aws.String("page-2")
			} else {
				first = 5
			}
			var reservations []types.Reservation
			for i := first; i < first+5; i++ {
				reservations = append(reservations, types.Reservation{Instances: []types.Instance{
					envInstance(fmt.Sprintf("i-%d", i), fmt.Sprintf("env%d", i), "running"),
				}})
			}
			return &ec2.DescribeInstancesOutput{Reservations: reservations, NextToken: next}, nil
		},
		describeVolumesFn: func(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
			volumeCalls++
			return &ec2.DescribeVolumesOutput{}, nil
		},
	}
	client := newEC2ClientWithAPI(mock)

	envs, err := client.ListEnvironments(context.Background(), "john-dev")
	if err != nil {
		t.Fatal(err)
	}
	volumeIDs := make([]string, 0, len(envs))
	for _, env := range envs {
		volumeIDs = append(volumeIDs, env.RootVolumeID)
	}
	if _, err := client.RootVolumeSizes(context.Background(), volumeIDs); err != nil {
		t.Fatal(err)
	}

	if len(envs) != 10 || describeCalls != 2 || volumeCalls != 1 {
		t.Errorf("envs=%d DescribeInstances=%d DescribeVolumes=%d, want 10, 2 and 1", len(envs), describeCalls, volumeCalls)
	}
}
