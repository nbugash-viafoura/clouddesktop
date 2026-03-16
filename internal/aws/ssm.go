package aws

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

const (
	ssmPollInitialInterval = 3 * time.Second
)

// ssmapi is the subset of the AWS SSM SDK client used by SSMClient.
type ssmapi interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	SendCommand(ctx context.Context, params *ssm.SendCommandInput, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error)
	GetCommandInvocation(ctx context.Context, params *ssm.GetCommandInvocationInput, optFns ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error)
}

// SSMClient wraps AWS SSM API operations for reading shared infrastructure parameters
// and sending remote commands to instances.
type SSMClient struct {
	client              ssmapi
	pollInitialInterval time.Duration
	pollMaxInterval     time.Duration
}

// SharedInfraConfig holds the shared infrastructure parameters set by Tier 1 Terraform.
type SharedInfraConfig struct {
	SubnetID            string
	SecurityGroupID     string
	InstanceProfileName string
}

const (
	ssmParamSubnetID            = "/clouddesktop/shared/subnet_id"
	ssmParamSecurityGroupID     = "/clouddesktop/shared/security_group_id"
	ssmParamInstanceProfileName = "/clouddesktop/shared/instance_profile_name"
)

// filesystemExtensionScript grows the root partition and resizes the ext4 filesystem.
// Handles both nvme (NVMe) and xvd (Xen) partition naming schemes.
const filesystemExtensionScript = `#!/usr/bin/env bash
set -euo pipefail

ROOT_PART=$(lsblk -rno NAME,MOUNTPOINT | awk '$2 == "/" {print $1}')
if [ -z "$ROOT_PART" ]; then echo "ERROR: could not detect root partition" >&2; exit 1; fi

if [[ "$ROOT_PART" =~ ^(nvme[0-9]+n[0-9]+)p([0-9]+)$ ]]; then
    DEVICE="/dev/${BASH_REMATCH[1]}"; PART_NUM="${BASH_REMATCH[2]}"
elif [[ "$ROOT_PART" =~ ^(xvd[a-z]+)([0-9]+)$ ]]; then
    DEVICE="/dev/${BASH_REMATCH[1]}"; PART_NUM="${BASH_REMATCH[2]}"
else echo "ERROR: unrecognized partition scheme: $ROOT_PART" >&2; exit 1; fi

sudo growpart "$DEVICE" "$PART_NUM"
sudo resize2fs "/dev/${ROOT_PART}"`

// NewSSMClient creates a new SSM client configured with the specified AWS profile and region.
func NewSSMClient(ctx context.Context, profile, region string) (*SSMClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config for SSM: %w", err)
	}

	return &SSMClient{
		client:              ssm.NewFromConfig(cfg),
		pollInitialInterval: ssmPollInitialInterval,
		pollMaxInterval:     pollMaxInterval,
	}, nil
}

// newSSMClientWithAPI creates an SSMClient with a custom API implementation (for testing).
func newSSMClientWithAPI(api ssmapi) *SSMClient {
	return &SSMClient{
		client:              api,
		pollInitialInterval: ssmPollInitialInterval,
		pollMaxInterval:     pollMaxInterval,
	}
}

// GetSharedInfraConfig reads the shared infrastructure parameters from SSM Parameter Store.
// These parameters are set by the Tier 1 shared Terraform configuration.
func (c *SSMClient) GetSharedInfraConfig(ctx context.Context) (*SharedInfraConfig, error) {
	subnetID, err := c.getParameter(ctx, ssmParamSubnetID)
	if err != nil {
		return nil, fmt.Errorf("failed to read subnet ID: %w", err)
	}

	sgID, err := c.getParameter(ctx, ssmParamSecurityGroupID)
	if err != nil {
		return nil, fmt.Errorf("failed to read security group ID: %w", err)
	}

	profileName, err := c.getParameter(ctx, ssmParamInstanceProfileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read instance profile name: %w", err)
	}

	return &SharedInfraConfig{
		SubnetID:            subnetID,
		SecurityGroupID:     sgID,
		InstanceProfileName: profileName,
	}, nil
}

// getParameter reads a single SSM parameter value.
func (c *SSMClient) getParameter(ctx context.Context, name string) (string, error) {
	output, err := c.client.GetParameter(ctx, &ssm.GetParameterInput{
		Name: &name,
	})
	if err != nil {
		return "", fmt.Errorf("SSM parameter %s: %w", name, err)
	}
	if output.Parameter == nil || output.Parameter.Value == nil {
		return "", fmt.Errorf("SSM parameter %s has no value", name)
	}
	return *output.Parameter.Value, nil
}

// isInvocationNotFoundError reports whether the error is the SSM InvocationDoesNotExist
// transient condition. SSM creates the invocation record asynchronously after SendCommand
// returns, so the first few GetCommandInvocation calls may return this error.
func isInvocationNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "InvocationDoesNotExist")
}

// RunFilesystemExtension sends AWS-RunShellScript to grow the root partition
// and resize the ext4 filesystem. Returns the SSM command ID.
func (c *SSMClient) RunFilesystemExtension(ctx context.Context, instanceID string) (string, error) {
	output, err := c.client.SendCommand(ctx, &ssm.SendCommandInput{
		DocumentName: strPtr("AWS-RunShellScript"),
		InstanceIds:  []string{instanceID},
		Parameters: map[string][]string{
			"commands": {filesystemExtensionScript},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to send filesystem extension command to instance %s: %w", instanceID, err)
	}

	if output.Command == nil || output.Command.CommandId == nil {
		return "", fmt.Errorf("SendCommand returned no command ID")
	}

	return *output.Command.CommandId, nil
}

// WaitUntilCommandComplete polls GetCommandInvocation until a terminal status
// (Success, Failed, TimedOut, Cancelled, Undeliverable, Terminated) is reached.
// Non-terminal statuses (Pending, InProgress, Delayed) cause continued polling.
func (c *SSMClient) WaitUntilCommandComplete(ctx context.Context, instanceID, commandID string) error {
	interval := c.pollInitialInterval
	consecutiveErrors := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		output, err := c.client.GetCommandInvocation(ctx, &ssm.GetCommandInvocationInput{
			CommandId:  &commandID,
			InstanceId: &instanceID,
		})
		if err != nil {
			if isTransientError(err) || isInvocationNotFoundError(err) {
				consecutiveErrors++
				if consecutiveErrors >= pollMaxConsecutiveErrors {
					return fmt.Errorf("too many errors waiting for command %s to complete: %w", commandID, err)
				}
			} else {
				return fmt.Errorf("failed to get command invocation for %s: %w", commandID, err)
			}
		} else {
			consecutiveErrors = 0
			switch output.Status {
			case ssmtypes.CommandInvocationStatusSuccess:
				return nil
			case ssmtypes.CommandInvocationStatusFailed,
				ssmtypes.CommandInvocationStatusTimedOut,
				ssmtypes.CommandInvocationStatusCancelled,
				ssmtypes.CommandInvocationStatusCancelling:
				return fmt.Errorf("SSM command %s finished with status: %s", commandID, output.Status)
			}
		}

		jitter := time.Duration(rand.Int63n(int64(interval / 4)))
		sleep := interval + jitter

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleep):
		}

		interval = interval * 2
		if interval > c.pollMaxInterval {
			interval = c.pollMaxInterval
		}
	}
}
