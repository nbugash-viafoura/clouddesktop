package aws

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

const (
	ssmPollInitialInterval = 3 * time.Second

	// ssmSendRetryLimit bounds how long SendCommand is retried while an
	// instance's SSM agent reconnects after a start.
	ssmSendRetryLimit = 3 * time.Minute
)

// ssmapi is the subset of the AWS SSM SDK client used by SSMClient.
type ssmapi interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	PutParameter(ctx context.Context, params *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
	DeleteParameter(ctx context.Context, params *ssm.DeleteParameterInput, optFns ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error)
	SendCommand(ctx context.Context, params *ssm.SendCommandInput, optFns ...func(*ssm.Options)) (*ssm.SendCommandOutput, error)
	GetCommandInvocation(ctx context.Context, params *ssm.GetCommandInvocationInput, optFns ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error)
}

// SSMClient wraps AWS SSM API operations for reading shared infrastructure parameters
// and sending remote commands to instances.
type SSMClient struct {
	client              ssmapi
	pollInitialInterval time.Duration
	pollMaxInterval     time.Duration

	sendRetryInitialInterval time.Duration
	sendRetryMaxInterval     time.Duration
	sendRetryLimit           time.Duration
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

	ssmParamDeveloperS3Bucket = "/clouddesktop/developer/%s/s3_bucket_name"
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
		client:                   ssm.NewFromConfig(cfg),
		pollInitialInterval:      ssmPollInitialInterval,
		pollMaxInterval:          pollMaxInterval,
		sendRetryInitialInterval: pollInitialInterval,
		sendRetryMaxInterval:     pollMaxInterval,
		sendRetryLimit:           ssmSendRetryLimit,
	}, nil
}

// newSSMClientWithAPI creates an SSMClient with a custom API implementation (for testing).
func newSSMClientWithAPI(api ssmapi) *SSMClient {
	return &SSMClient{
		client:                   api,
		pollInitialInterval:      ssmPollInitialInterval,
		pollMaxInterval:          pollMaxInterval,
		sendRetryInitialInterval: pollInitialInterval,
		sendRetryMaxInterval:     pollMaxInterval,
		sendRetryLimit:           ssmSendRetryLimit,
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

// S3BucketParamName returns the SSM parameter path for a developer's S3 bucket.
func S3BucketParamName(developerName string) string {
	return fmt.Sprintf(ssmParamDeveloperS3Bucket, developerName)
}

// GetDeveloperS3Bucket reads the S3 bucket name for a developer from SSM.
// Returns empty string and nil error if the parameter does not exist.
func (c *SSMClient) GetDeveloperS3Bucket(ctx context.Context, developerName string) (string, error) {
	paramName := S3BucketParamName(developerName)
	value, err := c.getParameter(ctx, paramName)
	if err != nil {
		if isParameterNotFoundError(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read S3 bucket param for %s: %w", developerName, err)
	}
	return value, nil
}

// PutDeveloperS3Bucket stores the S3 bucket name for a developer in SSM.
func (c *SSMClient) PutDeveloperS3Bucket(ctx context.Context, developerName, bucketName string) error {
	paramName := S3BucketParamName(developerName)
	return c.putParameter(ctx, paramName, bucketName)
}

// DeleteDeveloperS3Bucket removes the S3 bucket parameter for a developer from SSM.
func (c *SSMClient) DeleteDeveloperS3Bucket(ctx context.Context, developerName string) error {
	paramName := S3BucketParamName(developerName)
	return c.deleteParameter(ctx, paramName)
}

// putParameter writes a single SSM parameter value (overwrite if exists).
func (c *SSMClient) putParameter(ctx context.Context, name, value string) error {
	overwrite := true
	_, err := c.client.PutParameter(ctx, &ssm.PutParameterInput{
		Name:      &name,
		Value:     &value,
		Type:      ssmtypes.ParameterTypeString,
		Overwrite: &overwrite,
	})
	if err != nil {
		return fmt.Errorf("failed to put SSM parameter %s: %w", name, err)
	}
	return nil
}

// deleteParameter removes a single SSM parameter. Returns nil if the parameter does not exist.
func (c *SSMClient) deleteParameter(ctx context.Context, name string) error {
	_, err := c.client.DeleteParameter(ctx, &ssm.DeleteParameterInput{
		Name: &name,
	})
	if err != nil {
		if isParameterNotFoundError(err) {
			return nil
		}
		return fmt.Errorf("failed to delete SSM parameter %s: %w", name, err)
	}
	return nil
}

// isParameterNotFoundError checks if the error indicates the SSM parameter does not exist.
func isParameterNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "ParameterNotFound")
}

// s3MountScript installs mount-s3 if needed and mounts the bucket at
// /home/ubuntu/s3, adding an fstab entry so the mount survives reboots. It does
// nothing when the right bucket is already mounted and responding, and fails
// with "bootstrap in progress" on stderr while first-boot setup is still running.
const s3MountScript = `#!/usr/bin/env bash
set -euo pipefail

BUCKET=%s
MOUNT_DIR=/home/ubuntu/s3

if [[ ! -f /var/log/bootstrap-system-complete ]]; then
    echo "bootstrap in progress" >&2
    exit 1
fi

# Nothing to do when the right bucket is already mounted and responding.
if mountpoint -q "$MOUNT_DIR" && timeout 10 ls "$MOUNT_DIR" >/dev/null 2>&1 && grep -q "^${BUCKET} ${MOUNT_DIR} " /etc/fstab; then
    exit 0
fi

# Install Mountpoint for Amazon S3 if not present
if ! command -v mount-s3 &>/dev/null; then
    curl -fsSL -o /tmp/mount-s3.deb "https://s3.amazonaws.com/mountpoint-s3-release/latest/x86_64/mount-s3.deb"
    apt-get install -y /tmp/mount-s3.deb
    rm -f /tmp/mount-s3.deb
fi

# Enable FUSE allow_other
grep -q user_allow_other /etc/fuse.conf || echo user_allow_other >> /etc/fuse.conf

# Unmount a stale FUSE mount if present
if mountpoint -q "$MOUNT_DIR" 2>/dev/null; then
    umount "$MOUNT_DIR" || fusermount -u "$MOUNT_DIR" || true
fi

# Create mount point and set ownership
mkdir -p "$MOUNT_DIR"
chown ubuntu:ubuntu "$MOUNT_DIR"

# Mount the bucket
mount-s3 --allow-delete --allow-overwrite --allow-other --uid 1000 --gid 1000 "$BUCKET" "$MOUNT_DIR"

# Remove any stale fstab entries for the mount point
sed -i "\\|${MOUNT_DIR}|d" /etc/fstab

# Add fstab entry for auto-mount on boot
echo "${BUCKET} ${MOUNT_DIR} fuse.mount-s3 _netdev,allow_other,allow_delete,allow_overwrite,uid=1000,gid=1000 0 0" >> /etc/fstab`

// RunS3Mount sends a command that makes sure the bucket is mounted on the instance.
func (c *SSMClient) RunS3Mount(ctx context.Context, instanceID, bucketName string) (string, error) {
	commandID, err := c.sendCommand(ctx, instanceID, fmt.Sprintf(s3MountScript, bucketName))
	if err != nil {
		return "", fmt.Errorf("failed to send S3 mount command to instance %s: %w", instanceID, err)
	}
	return commandID, nil
}

// sendCommand runs a shell script on the instance through AWS-RunShellScript.
// Right after an instance starts, its SSM agent may not have reconnected yet and
// SendCommand fails with InvalidInstanceId, so that error is retried with
// exponential backoff until sendRetryLimit has passed.
func (c *SSMClient) sendCommand(ctx context.Context, instanceID, script string) (string, error) {
	deadline := time.Now().Add(c.sendRetryLimit)
	interval := c.sendRetryInitialInterval
	announced := false

	for {
		output, err := c.client.SendCommand(ctx, &ssm.SendCommandInput{
			DocumentName: strPtr("AWS-RunShellScript"),
			InstanceIds:  []string{instanceID},
			Parameters: map[string][]string{
				"commands": {script},
			},
		})
		if err == nil {
			if output.Command == nil || output.Command.CommandId == nil {
				return "", fmt.Errorf("SendCommand returned no command ID")
			}
			return *output.Command.CommandId, nil
		}
		if !isInstanceNotReachableError(err) {
			return "", err
		}
		if time.Now().Add(interval).After(deadline) {
			return "", fmt.Errorf("SSM can't reach instance %s yet: %w", instanceID, err)
		}
		if !announced {
			fmt.Println("Waiting for SSM to reach the instance...")
			announced = true
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}

		interval *= 2
		if interval > c.sendRetryMaxInterval {
			interval = c.sendRetryMaxInterval
		}
	}
}

// isInstanceNotReachableError reports whether SendCommand failed because the
// instance's SSM agent is not connected yet.
func isInstanceNotReachableError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "InvalidInstanceId")
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
	commandID, err := c.sendCommand(ctx, instanceID, filesystemExtensionScript)
	if err != nil {
		return "", fmt.Errorf("failed to send filesystem extension command to instance %s: %w", instanceID, err)
	}
	return commandID, nil
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
				if detail := strings.TrimSpace(aws.ToString(output.StandardErrorContent)); detail != "" {
					return fmt.Errorf("SSM command %s finished with status: %s: %s", commandID, output.Status, detail)
				}
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
