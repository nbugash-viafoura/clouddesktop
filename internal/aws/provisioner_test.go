package aws

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	s3sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// mockSSMForProvisioner implements ssmapi for provisioner tests.
type mockSSMForProvisioner struct {
	params map[string]string
	err    error
	sentTo []string // instance IDs that received a command
}

func (m *mockSSMForProvisioner) GetParameter(_ context.Context, input *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	name := *input.Name
	val, ok := m.params[name]
	if !ok {
		return nil, errors.New("ParameterNotFound: " + name)
	}
	return &ssm.GetParameterOutput{
		Parameter: &ssmtypes.Parameter{
			Name:  &name,
			Value: &val,
		},
	}, nil
}

func (m *mockSSMForProvisioner) PutParameter(_ context.Context, input *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.params[*input.Name] = *input.Value
	return &ssm.PutParameterOutput{}, nil
}

func (m *mockSSMForProvisioner) DeleteParameter(_ context.Context, input *ssm.DeleteParameterInput, _ ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	delete(m.params, *input.Name)
	return &ssm.DeleteParameterOutput{}, nil
}

func (m *mockSSMForProvisioner) SendCommand(_ context.Context, input *ssm.SendCommandInput, _ ...func(*ssm.Options)) (*ssm.SendCommandOutput, error) {
	m.sentTo = append(m.sentTo, input.InstanceIds...)
	return &ssm.SendCommandOutput{Command: &ssmtypes.Command{CommandId: aws.String("cmd-mount")}}, nil
}

func (m *mockSSMForProvisioner) GetCommandInvocation(_ context.Context, _ *ssm.GetCommandInvocationInput, _ ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error) {
	return &ssm.GetCommandInvocationOutput{Status: ssmtypes.CommandInvocationStatusSuccess}, nil
}

func defaultSSMMock() *mockSSMForProvisioner {
	return &mockSSMForProvisioner{
		params: map[string]string{
			"/clouddesktop/shared/subnet_id":             "subnet-abc123",
			"/clouddesktop/shared/security_group_id":     "sg-def456",
			"/clouddesktop/shared/instance_profile_name": "clouddesktop-developer-instance",
		},
	}
}

// mockS3ForProvisioner implements s3api for provisioner tests.
type mockS3ForProvisioner struct {
	buckets map[string]bool
}

func (m *mockS3ForProvisioner) CreateBucket(_ context.Context, input *s3sdk.CreateBucketInput, _ ...func(*s3sdk.Options)) (*s3sdk.CreateBucketOutput, error) {
	if m.buckets == nil {
		m.buckets = make(map[string]bool)
	}
	m.buckets[*input.Bucket] = true
	return &s3sdk.CreateBucketOutput{}, nil
}

func (m *mockS3ForProvisioner) HeadBucket(_ context.Context, input *s3sdk.HeadBucketInput, _ ...func(*s3sdk.Options)) (*s3sdk.HeadBucketOutput, error) {
	if m.buckets != nil && m.buckets[*input.Bucket] {
		return &s3sdk.HeadBucketOutput{}, nil
	}
	return nil, errors.New("not found")
}

func (m *mockS3ForProvisioner) ListObjectsV2(_ context.Context, _ *s3sdk.ListObjectsV2Input, _ ...func(*s3sdk.Options)) (*s3sdk.ListObjectsV2Output, error) {
	return &s3sdk.ListObjectsV2Output{}, nil
}

func (m *mockS3ForProvisioner) DeleteObjects(_ context.Context, _ *s3sdk.DeleteObjectsInput, _ ...func(*s3sdk.Options)) (*s3sdk.DeleteObjectsOutput, error) {
	return &s3sdk.DeleteObjectsOutput{}, nil
}

func (m *mockS3ForProvisioner) DeleteBucket(_ context.Context, input *s3sdk.DeleteBucketInput, _ ...func(*s3sdk.Options)) (*s3sdk.DeleteBucketOutput, error) {
	if m.buckets != nil {
		delete(m.buckets, *input.Bucket)
	}
	return &s3sdk.DeleteBucketOutput{}, nil
}

func newMockS3Client() *S3Client {
	return &S3Client{
		client: &mockS3ForProvisioner{buckets: make(map[string]bool)},
		region: "us-east-1",
	}
}

// amiFn returns a single Ubuntu image for provisioning tests.
func amiFn(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	return &ec2.DescribeImagesOutput{
		Images: []types.Image{
			{ImageId: aws.String("ami-ubuntu-latest"), CreationDate: aws.String("2024-06-01T00:00:00Z")},
		},
	}, nil
}

// listing returns a describeFn that reports the given instances for every call.
func listing(instances ...types.Instance) func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
		reservations := make([]types.Reservation, 0, len(instances))
		for _, inst := range instances {
			reservations = append(reservations, types.Reservation{Instances: []types.Instance{inst}})
		}
		return &ec2.DescribeInstancesOutput{Reservations: reservations}, nil
	}
}

func TestProvision_NewInstance(t *testing.T) {
	var runInput *ec2.RunInstancesInput
	ec2Mock := &mockEC2API{
		describeFn:    listing(),
		describeImgFn: amiFn,
		runFn: func(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
			runInput = params
			return &ec2.RunInstancesOutput{
				Instances: []types.Instance{
					{InstanceId: aws.String("i-new456")},
				},
			}, nil
		},
	}

	ec2Client := newEC2ClientWithAPI(ec2Mock)
	ssmClient := newSSMClientWithAPI(defaultSSMMock())
	s3Client := newMockS3Client()
	provisioner := NewProvisioner(ec2Client, ssmClient, s3Client)

	result, err := provisioner.Provision(context.Background(), ProvisionParams{
		DeveloperName:    "john-dev",
		EnvironmentName:  "build",
		InstanceType:     "m7i.xlarge",
		RootVolumeSizeGB: 300,
		SSHPublicKey:     "ssh-ed25519 AAAA... test@host",
		UserData:         []byte("#!/bin/bash\necho hello"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.InstanceID != "i-new456" {
		t.Errorf("InstanceID = %q, want i-new456", result.InstanceID)
	}
	if result.Recovered {
		t.Error("expected Recovered=false for new instance")
	}
	if aws.ToString(runInput.ImageId) != "ami-ubuntu-latest" {
		t.Errorf("AMI = %q, want ami-ubuntu-latest", aws.ToString(runInput.ImageId))
	}
	if aws.ToString(runInput.SubnetId) != "subnet-abc123" {
		t.Errorf("SubnetID = %q, want subnet-abc123", aws.ToString(runInput.SubnetId))
	}
	if size := aws.ToInt32(runInput.BlockDeviceMappings[0].Ebs.VolumeSize); size != 300 {
		t.Errorf("VolumeSize = %d, want 300", size)
	}
	if got := tagMap(runInput.TagSpecifications[0].Tags)["CloudDesktopEnv"]; got != "build" {
		t.Errorf("CloudDesktopEnv tag = %q, want build", got)
	}
	if !s3Client.client.(*mockS3ForProvisioner).buckets["clouddesktop-john-dev"] {
		t.Error("expected the developer bucket to be created")
	}
}

func TestProvision_RecoversMatchingEnvironment(t *testing.T) {
	ec2Mock := &mockEC2API{
		describeFn: listing(envInstance("i-main", "main", "running"), envInstance("i-build", "build", "stopped")),
	}

	provisioner := NewProvisioner(newEC2ClientWithAPI(ec2Mock), newSSMClientWithAPI(defaultSSMMock()), nil)

	result, err := provisioner.Provision(context.Background(), ProvisionParams{
		DeveloperName:   "john-dev",
		EnvironmentName: "build",
		InstanceType:    "m7i.xlarge",
		SSHPublicKey:    "ssh-ed25519 AAAA... test@host",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.InstanceID != "i-build" || !result.Recovered {
		t.Errorf("got %+v, want recovered i-build", result)
	}
}

func TestProvision_IgnoresOtherEnvironments(t *testing.T) {
	launched := false
	ec2Mock := &mockEC2API{
		describeFn:    listing(envInstance("i-main", "main", "running")),
		describeImgFn: amiFn,
		runFn: func(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
			launched = true
			return &ec2.RunInstancesOutput{Instances: []types.Instance{{InstanceId: aws.String("i-build")}}}, nil
		},
	}

	provisioner := NewProvisioner(newEC2ClientWithAPI(ec2Mock), newSSMClientWithAPI(defaultSSMMock()), newMockS3Client())

	result, err := provisioner.Provision(context.Background(), ProvisionParams{
		DeveloperName:   "john-dev",
		EnvironmentName: "build",
		InstanceType:    "m7i.xlarge",
		SSHPublicKey:    "ssh-ed25519 AAAA... test@host",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !launched || result.Recovered || result.InstanceID != "i-build" {
		t.Errorf("expected a new launch for build, got %+v (launched=%v)", result, launched)
	}
}

func TestProvision_RecoversLegacyInstanceAsMain(t *testing.T) {
	ec2Mock := &mockEC2API{
		describeFn: listing(envInstance("i-legacy", "", "stopped")),
	}

	provisioner := NewProvisioner(newEC2ClientWithAPI(ec2Mock), newSSMClientWithAPI(defaultSSMMock()), nil)

	result, err := provisioner.Provision(context.Background(), ProvisionParams{
		DeveloperName:   "john-dev",
		EnvironmentName: "main",
		InstanceType:    "m7i.xlarge",
		SSHPublicKey:    "ssh-ed25519 AAAA... test@host",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.InstanceID != "i-legacy" || !result.Recovered {
		t.Errorf("got %+v, want recovered i-legacy", result)
	}
}

func TestProvision_SSMError(t *testing.T) {
	ec2Mock := &mockEC2API{describeFn: listing()}

	ssmMock := &mockSSMForProvisioner{err: errors.New("AccessDeniedException")}

	ec2Client := newEC2ClientWithAPI(ec2Mock)
	ssmClient := newSSMClientWithAPI(ssmMock)
	provisioner := NewProvisioner(ec2Client, ssmClient, nil)

	_, err := provisioner.Provision(context.Background(), ProvisionParams{
		DeveloperName:   "john-dev",
		EnvironmentName: "main",
		InstanceType:    "m7i.xlarge",
		SSHPublicKey:    "ssh-ed25519 AAAA... test@host",
	})
	if err == nil {
		t.Fatal("expected error for SSM failure")
	}
	if !strings.Contains(err.Error(), "shared infrastructure config") {
		t.Errorf("error = %q, want to contain 'shared infrastructure config'", err)
	}
}

func TestProvision_RunInstanceFailure_KeepsSharedKeyPair(t *testing.T) {
	deleteKeyCalls := 0
	ec2Mock := &mockEC2API{
		describeFn:    listing(),
		describeImgFn: amiFn,
		runFn: func(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
			return nil, errors.New("InvalidParameterValue")
		},
		deleteKeyFn: func(ctx context.Context, params *ec2.DeleteKeyPairInput, optFns ...func(*ec2.Options)) (*ec2.DeleteKeyPairOutput, error) {
			deleteKeyCalls++
			return &ec2.DeleteKeyPairOutput{}, nil
		},
	}

	provisioner := NewProvisioner(newEC2ClientWithAPI(ec2Mock), newSSMClientWithAPI(defaultSSMMock()), newMockS3Client())

	_, err := provisioner.Provision(context.Background(), ProvisionParams{
		DeveloperName:   "john-dev",
		EnvironmentName: "build",
		InstanceType:    "m7i.xlarge",
		SSHPublicKey:    "ssh-ed25519 AAAA... test@host",
		UserData:        []byte("#!/bin/bash"),
	})
	if err == nil {
		t.Fatal("expected error for RunInstance failure")
	}
	// ImportSSHKeyPair deletes before importing; nothing may delete it after a failed launch.
	if deleteKeyCalls != 1 {
		t.Errorf("DeleteKeyPair calls = %d, want 1 (the import's own delete only)", deleteKeyCalls)
	}
}

// destroyMocks returns an EC2 mock that records terminate and key pair deletion,
// plus an SSM and S3 setup that already holds the developer's bucket.
func destroyMocks(terminated, deletedKey *bool) (*mockEC2API, *mockSSMForProvisioner, *S3Client) {
	ec2Mock := &mockEC2API{
		terminateFn: func(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {
			*terminated = params.InstanceIds[0] == "i-123"
			return &ec2.TerminateInstancesOutput{}, nil
		},
		deleteKeyFn: func(ctx context.Context, params *ec2.DeleteKeyPairInput, optFns ...func(*ec2.Options)) (*ec2.DeleteKeyPairOutput, error) {
			*deletedKey = aws.ToString(params.KeyName) == "clouddesktop-john-dev"
			return &ec2.DeleteKeyPairOutput{}, nil
		},
	}
	ssmMock := defaultSSMMock()
	ssmMock.params[S3BucketParamName("john-dev")] = "clouddesktop-john-dev"
	s3Client := newMockS3Client()
	s3Client.client.(*mockS3ForProvisioner).buckets["clouddesktop-john-dev"] = true
	return ec2Mock, ssmMock, s3Client
}

func TestDestroy_KeepsKeyPairAndBucketWhenOthersRemain(t *testing.T) {
	var terminated, deletedKey bool
	ec2Mock, ssmMock, s3Client := destroyMocks(&terminated, &deletedKey)
	provisioner := NewProvisioner(newEC2ClientWithAPI(ec2Mock), newSSMClientWithAPI(ssmMock), s3Client)

	if err := provisioner.Destroy(context.Background(), "i-123", "john-dev", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !terminated {
		t.Error("expected TerminateInstances to be called")
	}
	if deletedKey {
		t.Error("key pair must be kept while other environments remain")
	}
	if !s3Client.client.(*mockS3ForProvisioner).buckets["clouddesktop-john-dev"] {
		t.Error("bucket must never be deleted by destroy")
	}
	if _, ok := ssmMock.params[S3BucketParamName("john-dev")]; !ok {
		t.Error("bucket parameter must never be deleted by destroy")
	}
}

func TestDestroy_DeletesKeyPairForLastEnvironment(t *testing.T) {
	var terminated, deletedKey bool
	ec2Mock, ssmMock, s3Client := destroyMocks(&terminated, &deletedKey)
	provisioner := NewProvisioner(newEC2ClientWithAPI(ec2Mock), newSSMClientWithAPI(ssmMock), s3Client)

	if err := provisioner.Destroy(context.Background(), "i-123", "john-dev", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !terminated || !deletedKey {
		t.Errorf("terminated=%v deletedKey=%v, want both true", terminated, deletedKey)
	}
	if !s3Client.client.(*mockS3ForProvisioner).buckets["clouddesktop-john-dev"] {
		t.Error("bucket must never be deleted by destroy")
	}
}

func TestDestroy_TerminateError(t *testing.T) {
	ec2Mock := &mockEC2API{
		terminateFn: func(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {
			return nil, errors.New("access denied")
		},
	}

	ec2Client := newEC2ClientWithAPI(ec2Mock)
	ssmClient := newSSMClientWithAPI(defaultSSMMock())
	provisioner := NewProvisioner(ec2Client, ssmClient, nil)

	err := provisioner.Destroy(context.Background(), "i-123", "john-dev", false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to terminate instance") {
		t.Errorf("error = %q, want to contain 'failed to terminate instance'", err)
	}
}

func TestResize_Success(t *testing.T) {
	modifyCalled := false
	ec2Mock := &mockEC2API{
		modifyAttrFn: func(ctx context.Context, params *ec2.ModifyInstanceAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyInstanceAttributeOutput, error) {
			modifyCalled = true
			return &ec2.ModifyInstanceAttributeOutput{}, nil
		},
	}

	ec2Client := newEC2ClientWithAPI(ec2Mock)
	ssmClient := newSSMClientWithAPI(defaultSSMMock())
	provisioner := NewProvisioner(ec2Client, ssmClient, nil)

	err := provisioner.Resize(context.Background(), "i-123", "m7i.2xlarge")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !modifyCalled {
		t.Error("expected ModifyInstanceAttribute to be called")
	}
}

func TestEnsureS3Mount_CreatesMissingBucketThenMounts(t *testing.T) {
	ssmMock := defaultSSMMock()
	s3Client := newMockS3Client()
	provisioner := NewProvisioner(nil, newSSMClientWithAPI(ssmMock), s3Client)

	if err := provisioner.EnsureS3Mount(context.Background(), "i-build", "john-dev"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !s3Client.client.(*mockS3ForProvisioner).buckets["clouddesktop-john-dev"] {
		t.Error("expected the bucket to be created")
	}
	if ssmMock.params[S3BucketParamName("john-dev")] != "clouddesktop-john-dev" {
		t.Error("expected the bucket parameter to be stored")
	}
	if len(ssmMock.sentTo) != 1 || ssmMock.sentTo[0] != "i-build" {
		t.Errorf("mount command sent to %v, want [i-build]", ssmMock.sentTo)
	}
}

func TestEnsureS3Mount_RecreatesDeletedBucket(t *testing.T) {
	ssmMock := defaultSSMMock()
	ssmMock.params[S3BucketParamName("john-dev")] = "clouddesktop-john-dev"
	s3Client := newMockS3Client() // the parameter exists but the bucket does not
	provisioner := NewProvisioner(nil, newSSMClientWithAPI(ssmMock), s3Client)

	if err := provisioner.EnsureS3Mount(context.Background(), "i-main", "john-dev"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !s3Client.client.(*mockS3ForProvisioner).buckets["clouddesktop-john-dev"] {
		t.Error("expected the deleted bucket to be recreated")
	}
}

// Regression test for the bug where a freshly provisioned instance was never
// mounted because the bucket and its parameter already existed.
func TestEnsureS3Mount_MountsEvenWhenBucketExists(t *testing.T) {
	ssmMock := defaultSSMMock()
	ssmMock.params[S3BucketParamName("john-dev")] = "clouddesktop-john-dev"
	s3Client := newMockS3Client()
	s3Client.client.(*mockS3ForProvisioner).buckets["clouddesktop-john-dev"] = true
	provisioner := NewProvisioner(nil, newSSMClientWithAPI(ssmMock), s3Client)

	if err := provisioner.EnsureS3Mount(context.Background(), "i-new", "john-dev"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ssmMock.sentTo) != 1 || ssmMock.sentTo[0] != "i-new" {
		t.Errorf("mount command sent to %v, want [i-new]", ssmMock.sentTo)
	}
}
