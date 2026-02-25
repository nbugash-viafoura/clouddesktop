package aws

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	pollInitialInterval = 2 * time.Second
	pollMaxInterval     = 15 * time.Second
	pollMaxConsecutiveErrors = 10
)

// ec2api is the subset of the AWS EC2 SDK client used by EC2Client.
type ec2api interface {
	StartInstances(ctx context.Context, params *ec2.StartInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StartInstancesOutput, error)
	StopInstances(ctx context.Context, params *ec2.StopInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StopInstancesOutput, error)
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	RunInstances(ctx context.Context, params *ec2.RunInstancesInput, optFns ...func(*ec2.Options)) (*ec2.RunInstancesOutput, error)
	TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput, optFns ...func(*ec2.Options)) (*ec2.TerminateInstancesOutput, error)
	ImportKeyPair(ctx context.Context, params *ec2.ImportKeyPairInput, optFns ...func(*ec2.Options)) (*ec2.ImportKeyPairOutput, error)
	DeleteKeyPair(ctx context.Context, params *ec2.DeleteKeyPairInput, optFns ...func(*ec2.Options)) (*ec2.DeleteKeyPairOutput, error)
	DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
	ModifyInstanceAttribute(ctx context.Context, params *ec2.ModifyInstanceAttributeInput, optFns ...func(*ec2.Options)) (*ec2.ModifyInstanceAttributeOutput, error)
}

// EC2Client wraps AWS EC2 API operations for managing cloud desktop instances.
type EC2Client struct {
	client              ec2api
	pollInitialInterval time.Duration
	pollMaxInterval     time.Duration
}

// InstanceInfo contains information about an EC2 instance.
type InstanceInfo struct {
	InstanceID   string
	State        string
	PrivateIP    string
	InstanceType string
	LaunchTime   *time.Time
}

// NewEC2Client creates a new EC2 client configured with the specified AWS profile and region.
func NewEC2Client(ctx context.Context, profile, region string) (*EC2Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, err
	}

	return &EC2Client{
		client:              ec2.NewFromConfig(cfg),
		pollInitialInterval: pollInitialInterval,
		pollMaxInterval:     pollMaxInterval,
	}, nil
}

// newEC2ClientWithAPI creates an EC2Client with a custom API implementation (for testing).
func newEC2ClientWithAPI(api ec2api) *EC2Client {
	return &EC2Client{
		client:              api,
		pollInitialInterval: pollInitialInterval,
		pollMaxInterval:     pollMaxInterval,
	}
}

// StartInstance starts a stopped EC2 instance.
func (c *EC2Client) StartInstance(ctx context.Context, instanceID string) error {
	_, err := c.client.StartInstances(ctx, &ec2.StartInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("failed to start instance %s: %w", instanceID, err)
	}
	return nil
}

// StopInstance stops a running EC2 instance.
func (c *EC2Client) StopInstance(ctx context.Context, instanceID string) error {
	_, err := c.client.StopInstances(ctx, &ec2.StopInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("failed to stop instance %s: %w", instanceID, err)
	}
	return nil
}

// DescribeInstance retrieves information about an EC2 instance.
func (c *EC2Client) DescribeInstance(ctx context.Context, instanceID string) (*InstanceInfo, error) {
	output, err := c.client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   strPtr("instance-id"),
				Values: []string{instanceID},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance %s: %w", instanceID, err)
	}

	if len(output.Reservations) == 0 || len(output.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf("instance %s not found", instanceID)
	}

	inst := output.Reservations[0].Instances[0]

	info := &InstanceInfo{
		InstanceID:   instanceID,
		State:        string(inst.State.Name),
		InstanceType: string(inst.InstanceType),
		LaunchTime:   inst.LaunchTime,
	}

	if inst.PrivateIpAddress != nil {
		info.PrivateIP = *inst.PrivateIpAddress
	}

	return info, nil
}

// RunInstanceParams holds the parameters for launching a new EC2 instance.
type RunInstanceParams struct {
	AMIID               string
	InstanceType        string
	SubnetID            string
	SecurityGroupID     string
	KeyName             string
	InstanceProfileName string
	UserData            []byte
	DeveloperName       string
}

// FindInstanceByDeveloper searches for a non-terminated EC2 instance belonging
// to the given developer. Returns nil, nil if no instance is found.
func (c *EC2Client) FindInstanceByDeveloper(ctx context.Context, developerName string) (*InstanceInfo, error) {
	output, err := c.client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{Name: strPtr("tag:Developer"), Values: []string{developerName}},
			{Name: strPtr("tag:Project"), Values: []string{"clouddesktop"}},
			{Name: strPtr("instance-state-name"), Values: []string{"pending", "running", "stopping", "stopped"}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find instance for developer %s: %w", developerName, err)
	}

	for _, reservation := range output.Reservations {
		for _, inst := range reservation.Instances {
			info := &InstanceInfo{
				InstanceID:   aws.ToString(inst.InstanceId),
				State:        string(inst.State.Name),
				InstanceType: string(inst.InstanceType),
				LaunchTime:   inst.LaunchTime,
			}
			if inst.PrivateIpAddress != nil {
				info.PrivateIP = *inst.PrivateIpAddress
			}
			return info, nil
		}
	}

	return nil, nil
}

// ImportSSHKeyPair imports an SSH public key as an EC2 key pair. If a key pair
// with the same name already exists, it is deleted first for idempotency.
func (c *EC2Client) ImportSSHKeyPair(ctx context.Context, keyName, publicKey string) error {
	// Delete existing key pair first (idempotent -- ignores NotFound).
	_, _ = c.client.DeleteKeyPair(ctx, &ec2.DeleteKeyPairInput{
		KeyName: &keyName,
	})

	_, err := c.client.ImportKeyPair(ctx, &ec2.ImportKeyPairInput{
		KeyName:           &keyName,
		PublicKeyMaterial: []byte(publicKey),
	})
	if err != nil {
		return fmt.Errorf("failed to import key pair %s: %w", keyName, err)
	}
	return nil
}

// DeleteSSHKeyPair deletes an EC2 key pair. Ignores errors if the key pair
// does not exist (best-effort cleanup).
func (c *EC2Client) DeleteSSHKeyPair(ctx context.Context, keyName string) error {
	_, err := c.client.DeleteKeyPair(ctx, &ec2.DeleteKeyPairInput{
		KeyName: &keyName,
	})
	if err != nil {
		return fmt.Errorf("failed to delete key pair %s: %w", keyName, err)
	}
	return nil
}

// TerminateInstance terminates an EC2 instance.
func (c *EC2Client) TerminateInstance(ctx context.Context, instanceID string) error {
	_, err := c.client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return fmt.Errorf("failed to terminate instance %s: %w", instanceID, err)
	}
	return nil
}

// ModifyInstanceType changes the instance type of a stopped EC2 instance.
func (c *EC2Client) ModifyInstanceType(ctx context.Context, instanceID, newType string) error {
	_, err := c.client.ModifyInstanceAttribute(ctx, &ec2.ModifyInstanceAttributeInput{
		InstanceId: &instanceID,
		InstanceType: &types.AttributeValue{
			Value: &newType,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to modify instance type for %s: %w", instanceID, err)
	}
	return nil
}

// FindUbuntuAMI finds the latest Ubuntu 24.04 amd64 AMI from Canonical.
func (c *EC2Client) FindUbuntuAMI(ctx context.Context) (string, error) {
	output, err := c.client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Owners: []string{"099720109477"}, // Canonical
		Filters: []types.Filter{
			{Name: strPtr("name"), Values: []string{"ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"}},
			{Name: strPtr("virtualization-type"), Values: []string{"hvm"}},
			{Name: strPtr("state"), Values: []string{"available"}},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to find Ubuntu AMI: %w", err)
	}

	if len(output.Images) == 0 {
		return "", fmt.Errorf("no Ubuntu 24.04 AMI found")
	}

	// Sort by creation date descending to get the latest.
	sort.Slice(output.Images, func(i, j int) bool {
		return aws.ToString(output.Images[i].CreationDate) > aws.ToString(output.Images[j].CreationDate)
	})

	return aws.ToString(output.Images[0].ImageId), nil
}

// RunInstance launches a new EC2 instance with the specified parameters.
// Returns the instance ID of the newly created instance.
func (c *EC2Client) RunInstance(ctx context.Context, params RunInstanceParams) (string, error) {
	tagSpec := []types.TagSpecification{
		{
			ResourceType: types.ResourceTypeInstance,
			Tags: []types.Tag{
				{Key: strPtr("Name"), Value: strPtr("clouddesktop-" + params.DeveloperName)},
				{Key: strPtr("Developer"), Value: strPtr(params.DeveloperName)},
				{Key: strPtr("Project"), Value: strPtr("clouddesktop")},
				{Key: strPtr("ManagedBy"), Value: strPtr("clouddesktop-cli")},
			},
		},
		{
			ResourceType: types.ResourceTypeVolume,
			Tags: []types.Tag{
				{Key: strPtr("Name"), Value: strPtr("clouddesktop-" + params.DeveloperName + "-root")},
				{Key: strPtr("Developer"), Value: strPtr(params.DeveloperName)},
			},
		},
	}

	var one int32 = 1
	var volumeSize int32 = 100
	var iops int32 = 3000
	var throughput int32 = 300
	var hopLimit int32 = 1

	input := &ec2.RunInstancesInput{
		ImageId:      &params.AMIID,
		InstanceType: types.InstanceType(params.InstanceType),
		MinCount:     &one,
		MaxCount:     &one,
		SubnetId:     &params.SubnetID,
		SecurityGroupIds: []string{params.SecurityGroupID},
		KeyName:      &params.KeyName,
		IamInstanceProfile: &types.IamInstanceProfileSpecification{
			Name: &params.InstanceProfileName,
		},
		UserData: aws.String(base64.StdEncoding.EncodeToString(params.UserData)),
		BlockDeviceMappings: []types.BlockDeviceMapping{
			{
				DeviceName: strPtr("/dev/sda1"),
				Ebs: &types.EbsBlockDevice{
					VolumeType:          types.VolumeTypeGp3,
					VolumeSize:          &volumeSize,
					Iops:                &iops,
					Throughput:          &throughput,
					DeleteOnTermination: aws.Bool(true),
					Encrypted:           aws.Bool(true),
				},
			},
		},
		MetadataOptions: &types.InstanceMetadataOptionsRequest{
			HttpEndpoint:            types.InstanceMetadataEndpointStateEnabled,
			HttpTokens:              types.HttpTokensStateRequired,
			HttpPutResponseHopLimit: &hopLimit,
			InstanceMetadataTags:    types.InstanceMetadataTagsStateEnabled,
		},
		TagSpecifications: tagSpec,
	}

	output, err := c.client.RunInstances(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to launch instance: %w", err)
	}

	if len(output.Instances) == 0 {
		return "", fmt.Errorf("RunInstances returned no instances")
	}

	return aws.ToString(output.Instances[0].InstanceId), nil
}

// WaitUntilRunning polls DescribeInstance with exponential backoff until the
// instance reaches the "running" state or the context is cancelled. Transient
// errors (network timeouts, throttling) are tolerated up to 3 consecutive
// failures before giving up.
func (c *EC2Client) WaitUntilRunning(ctx context.Context, instanceID string) error {
	return c.waitForState(ctx, instanceID, "running")
}

// WaitUntilStopped polls DescribeInstance with exponential backoff until the
// instance reaches the "stopped" state or the context is cancelled. Transient
// errors are tolerated the same as WaitUntilRunning.
func (c *EC2Client) WaitUntilStopped(ctx context.Context, instanceID string) error {
	return c.waitForState(ctx, instanceID, "stopped")
}

// waitForState polls DescribeInstance with exponential backoff and jitter until
// the instance reaches the target state. Transient errors are retried up to
// pollMaxConsecutiveErrors times before failing.
func (c *EC2Client) waitForState(ctx context.Context, instanceID, targetState string) error {
	interval := c.pollInitialInterval
	consecutiveErrors := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		info, err := c.DescribeInstance(ctx, instanceID)
		if err != nil {
			if isTransientError(err) || isNotFoundError(err) {
				consecutiveErrors++
				if consecutiveErrors >= pollMaxConsecutiveErrors {
					return fmt.Errorf("instance %s not found", instanceID)
				}
			} else {
				return err
			}
		} else {
			consecutiveErrors = 0
			if info.State == targetState {
				return nil
			}
		}

		// Exponential backoff with jitter
		jitter := time.Duration(rand.Int63n(int64(interval / 4)))
		sleep := interval + jitter

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleep):
		}

		// Increase interval up to max
		interval = interval * 2
		if interval > c.pollMaxInterval {
			interval = c.pollMaxInterval
		}
	}
}

// isTransientError checks whether an error is likely transient and safe to retry.
// Network timeouts, throttling, and internal service errors are transient.
// Permission-denied and not-found errors are not.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	transientPatterns := []string{
		"Throttling",
		"RequestLimitExceeded",
		"InternalError",
		"ServiceUnavailable",
		"RequestTimeout",
		"connection reset",
		"i/o timeout",
		"TLS handshake timeout",
	}
	for _, pattern := range transientPatterns {
		if strings.Contains(msg, pattern) {
			return true
		}
	}
	return false
}

// isNotFoundError checks if the error is a "not found" result from DescribeInstance.
// This is expected briefly after RunInstances due to AWS eventual consistency.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not found")
}

// strPtr returns a pointer to the given string value.
func strPtr(s string) *string {
	return &s
}
