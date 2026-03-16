package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// cloudwatchapi is the subset of the AWS CloudWatch SDK client used by CloudWatchClient.
type cloudwatchapi interface {
	GetMetricStatistics(ctx context.Context, params *cloudwatch.GetMetricStatisticsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricStatisticsOutput, error)
}

// CloudWatchClient wraps AWS CloudWatch API operations for retrieving instance metrics.
type CloudWatchClient struct {
	client cloudwatchapi
}

// InstanceMetrics contains resource utilization metrics for an instance.
// A value of -1 for MemoryPercent or DiskPercent indicates the metric is not
// available because the CloudWatch agent is not running on the instance.
type InstanceMetrics struct {
	CPUPercent    float64
	MemoryPercent float64
	DiskPercent   float64
}

// NewCloudWatchClient creates a new CloudWatch client configured with the specified AWS profile and region.
func NewCloudWatchClient(ctx context.Context, profile, region string) (*CloudWatchClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, err
	}

	return &CloudWatchClient{
		client: cloudwatch.NewFromConfig(cfg),
	}, nil
}

// newCloudWatchClientWithAPI creates a CloudWatchClient with a custom API implementation (for testing).
func newCloudWatchClientWithAPI(api cloudwatchapi) *CloudWatchClient {
	return &CloudWatchClient{client: api}
}

// GetInstanceMetrics retrieves current resource utilization metrics for an EC2 instance.
// Only CPUUtilization is populated; MemoryPercent and DiskPercent are set to -1
// because those metrics require the CloudWatch agent to be installed and running.
func (c *CloudWatchClient) GetInstanceMetrics(ctx context.Context, instanceID string) (*InstanceMetrics, error) {
	now := time.Now()
	startTime := now.Add(-10 * time.Minute)
	period := int32(300)
	namespace := "AWS/EC2"
	metricName := "CPUUtilization"
	dimensionName := "InstanceId"

	output, err := c.client.GetMetricStatistics(ctx, &cloudwatch.GetMetricStatisticsInput{
		Namespace:  &namespace,
		MetricName: &metricName,
		Dimensions: []types.Dimension{
			{
				Name:  &dimensionName,
				Value: &instanceID,
			},
		},
		Period:     &period,
		Statistics: []types.Statistic{types.StatisticAverage},
		StartTime:  &startTime,
		EndTime:    &now,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get CloudWatch metrics for instance %s: %w", instanceID, err)
	}

	metrics := &InstanceMetrics{
		CPUPercent:    0,
		MemoryPercent: -1,
		DiskPercent:   -1,
	}

	// Find the most recent datapoint.
	var latest *time.Time
	for i := range output.Datapoints {
		dp := &output.Datapoints[i]
		if latest == nil || dp.Timestamp.After(*latest) {
			latest = dp.Timestamp
			if dp.Average != nil {
				metrics.CPUPercent = *dp.Average
			}
		}
	}

	return metrics, nil
}
