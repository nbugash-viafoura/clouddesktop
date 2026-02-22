package aws

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	pollInitialInterval = 2 * time.Second
	pollMaxInterval     = 15 * time.Second
	pollMaxConsecutiveErrors = 3
)

// ec2api is the subset of the AWS EC2 SDK client used by EC2Client.
type ec2api interface {
	StartInstances(ctx context.Context, params *ec2.StartInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StartInstancesOutput, error)
	StopInstances(ctx context.Context, params *ec2.StopInstancesInput, optFns ...func(*ec2.Options)) (*ec2.StopInstancesOutput, error)
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
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
			if isTransientError(err) {
				consecutiveErrors++
				if consecutiveErrors >= pollMaxConsecutiveErrors {
					return fmt.Errorf("giving up after %d consecutive transient errors while waiting for instance %s to reach %q state: %w",
						consecutiveErrors, instanceID, targetState, err)
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

// strPtr returns a pointer to the given string value.
func strPtr(s string) *string {
	return &s
}
