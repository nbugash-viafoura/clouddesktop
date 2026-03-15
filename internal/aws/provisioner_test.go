package aws

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// mockSSMForProvisioner implements ssmapi for provisioner tests.
type mockSSMForProvisioner struct {
	params map[string]string
	err    error
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

func defaultSSMMock() *mockSSMForProvisioner {
	return &mockSSMForProvisioner{
		params: map[string]string{
			"/clouddesktop/shared/subnet_id":             "subnet-abc123",
			"/clouddesktop/shared/security_group_id":     "sg-def456",
			"/clouddesktop/shared/instance_profile_name": "clouddesktop-developer-instance",
		},
	}
}

func TestProvision_NewInstance(t *testing.T) {
	ec2Mock := &mockEC2API{
		// FindInstanceByDeveloper returns no results.
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{Reservations: []types.Reservation{}}, nil
		},
		describeImgFn: func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{
				Images: []types.Image{
					{ImageId: aws.String("ami-ubuntu-latest"), CreationDate: aws.String("2024-06-01T00:00:00Z")},
				},
			}, nil
		},
		runFn: func(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
			// Verify key params are passed through.
			if aws.ToString(params.ImageId) != "ami-ubuntu-latest" {
				t.Errorf("AMI = %q, want ami-ubuntu-latest", aws.ToString(params.ImageId))
			}
			if aws.ToString(params.SubnetId) != "subnet-abc123" {
				t.Errorf("SubnetID = %q, want subnet-abc123", aws.ToString(params.SubnetId))
			}
			return &ec2.RunInstancesOutput{
				Instances: []types.Instance{
					{InstanceId: aws.String("i-new456")},
				},
			}, nil
		},
	}

	ec2Client := newEC2ClientWithAPI(ec2Mock)
	ssmClient := newSSMClientWithAPI(defaultSSMMock())
	provisioner := NewProvisioner(ec2Client, ssmClient)

	result, err := provisioner.Provision(context.Background(), ProvisionParams{
		DeveloperName: "john-dev",
		InstanceType:  "m7i.xlarge",
		SSHPublicKey:  "ssh-ed25519 AAAA... test@host",
		UserData:      []byte("#!/bin/bash\necho hello"),
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
}

func TestProvision_RecoveredInstance(t *testing.T) {
	now := time.Now()
	ec2Mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{
								InstanceId:       aws.String("i-existing789"),
								State:            &types.InstanceState{Name: types.InstanceStateNameStopped},
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

	ec2Client := newEC2ClientWithAPI(ec2Mock)
	ssmClient := newSSMClientWithAPI(defaultSSMMock())
	provisioner := NewProvisioner(ec2Client, ssmClient)

	result, err := provisioner.Provision(context.Background(), ProvisionParams{
		DeveloperName: "john-dev",
		InstanceType:  "m7i.xlarge",
		SSHPublicKey:  "ssh-ed25519 AAAA... test@host",
		UserData:      []byte("#!/bin/bash\necho hello"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.InstanceID != "i-existing789" {
		t.Errorf("InstanceID = %q, want i-existing789", result.InstanceID)
	}
	if !result.Recovered {
		t.Error("expected Recovered=true for existing instance")
	}
}

func TestProvision_SSMError(t *testing.T) {
	ec2Mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{Reservations: []types.Reservation{}}, nil
		},
	}

	ssmMock := &mockSSMForProvisioner{err: errors.New("AccessDeniedException")}

	ec2Client := newEC2ClientWithAPI(ec2Mock)
	ssmClient := newSSMClientWithAPI(ssmMock)
	provisioner := NewProvisioner(ec2Client, ssmClient)

	_, err := provisioner.Provision(context.Background(), ProvisionParams{
		DeveloperName: "john-dev",
		InstanceType:  "m7i.xlarge",
		SSHPublicKey:  "ssh-ed25519 AAAA... test@host",
	})
	if err == nil {
		t.Fatal("expected error for SSM failure")
	}
	if !strings.Contains(err.Error(), "shared infrastructure config") {
		t.Errorf("error = %q, want to contain 'shared infrastructure config'", err)
	}
}

func TestProvision_RunInstanceFailure_CleansUpKeyPair(t *testing.T) {
	deleteKeyCalled := false
	ec2Mock := &mockEC2API{
		describeFn: func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{Reservations: []types.Reservation{}}, nil
		},
		describeImgFn: func(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{
				Images: []types.Image{
					{ImageId: aws.String("ami-test"), CreationDate: aws.String("2024-06-01T00:00:00Z")},
				},
			}, nil
		},
		runFn: func(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error) {
			return nil, errors.New("InsufficientInstanceCapacity")
		},
		deleteKeyFn: func(ctx context.Context, params *ec2.DeleteKeyPairInput, optFns ...func(*ec2.Options)) (*ec2.DeleteKeyPairOutput, error) {
			if aws.ToString(params.KeyName) == "clouddesktop-john-dev" {
				deleteKeyCalled = true
			}
			return &ec2.DeleteKeyPairOutput{}, nil
		},
	}

	ec2Client := newEC2ClientWithAPI(ec2Mock)
	ssmClient := newSSMClientWithAPI(defaultSSMMock())
	provisioner := NewProvisioner(ec2Client, ssmClient)

	_, err := provisioner.Provision(context.Background(), ProvisionParams{
		DeveloperName: "john-dev",
		InstanceType:  "m7i.xlarge",
		SSHPublicKey:  "ssh-ed25519 AAAA... test@host",
		UserData:      []byte("#!/bin/bash"),
	})
	if err == nil {
		t.Fatal("expected error for RunInstance failure")
	}
	if !deleteKeyCalled {
		t.Error("expected DeleteKeyPair to be called for cleanup on RunInstance failure")
	}
}

func TestDestroy_Success(t *testing.T) {
	terminateCalled := false
	deleteKeyCalled := false
	ec2Mock := &mockEC2API{
		terminateFn: func(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error) {
			if params.InstanceIds[0] == "i-123" {
				terminateCalled = true
			}
			return &ec2.TerminateInstancesOutput{}, nil
		},
		deleteKeyFn: func(ctx context.Context, params *ec2.DeleteKeyPairInput, optFns ...func(*ec2.Options)) (*ec2.DeleteKeyPairOutput, error) {
			if aws.ToString(params.KeyName) == "clouddesktop-john-dev" {
				deleteKeyCalled = true
			}
			return &ec2.DeleteKeyPairOutput{}, nil
		},
	}

	ec2Client := newEC2ClientWithAPI(ec2Mock)
	ssmClient := newSSMClientWithAPI(defaultSSMMock())
	provisioner := NewProvisioner(ec2Client, ssmClient)

	err := provisioner.Destroy(context.Background(), "i-123", "john-dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !terminateCalled {
		t.Error("expected TerminateInstances to be called")
	}
	if !deleteKeyCalled {
		t.Error("expected DeleteKeyPair to be called")
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
	provisioner := NewProvisioner(ec2Client, ssmClient)

	err := provisioner.Destroy(context.Background(), "i-123", "john-dev")
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
	provisioner := NewProvisioner(ec2Client, ssmClient)

	err := provisioner.Resize(context.Background(), "i-123", "m7i.2xlarge")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !modifyCalled {
		t.Error("expected ModifyInstanceAttribute to be called")
	}
}
