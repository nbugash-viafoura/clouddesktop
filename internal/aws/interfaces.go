package aws

import "context"

// EC2Service defines the interface for EC2 instance lifecycle operations.
// Implemented by EC2Client; mocked by testing.MockEC2Client.
type EC2Service interface {
	StartInstance(ctx context.Context, instanceID string) error
	StopInstance(ctx context.Context, instanceID string) error
	DescribeInstance(ctx context.Context, instanceID string) (*InstanceInfo, error)
	WaitUntilRunning(ctx context.Context, instanceID string) error
	WaitUntilStopped(ctx context.Context, instanceID string) error
	GetRootVolumeInfo(ctx context.Context, instanceID string) (volumeID string, sizeGB int32, err error)
	ModifyRootVolume(ctx context.Context, volumeID string, newSizeGB int32) error
	WaitUntilVolumeResized(ctx context.Context, volumeID string) error
}

// CloudWatchService defines the interface for retrieving instance metrics.
// Implemented by CloudWatchClient; mocked by testing.MockCloudWatchClient.
type CloudWatchService interface {
	GetInstanceMetrics(ctx context.Context, instanceID string) (*InstanceMetrics, error)
}

// SSMService defines the interface for SSM remote command operations.
// Implemented by SSMClient; mocked by testing.MockSSMClient.
type SSMService interface {
	RunFilesystemExtension(ctx context.Context, instanceID string) (commandID string, err error)
	WaitUntilCommandComplete(ctx context.Context, instanceID, commandID string) error
}
