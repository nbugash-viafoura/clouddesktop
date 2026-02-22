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
}

// CloudWatchService defines the interface for retrieving instance metrics.
// Implemented by CloudWatchClient; mocked by testing.MockCloudWatchClient.
type CloudWatchService interface {
	GetInstanceMetrics(ctx context.Context, instanceID string) (*InstanceMetrics, error)
}
