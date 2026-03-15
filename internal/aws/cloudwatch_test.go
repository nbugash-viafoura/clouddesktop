package aws

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

type mockCloudWatchAPI struct {
	fn func(ctx context.Context, params *cloudwatch.GetMetricStatisticsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricStatisticsOutput, error)
}

func (m *mockCloudWatchAPI) GetMetricStatistics(ctx context.Context, params *cloudwatch.GetMetricStatisticsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricStatisticsOutput, error) {
	return m.fn(ctx, params, optFns...)
}

func TestGetInstanceMetrics_Success(t *testing.T) {
	older := time.Now().Add(-5 * time.Minute)
	newer := time.Now().Add(-1 * time.Minute)
	olderAvg := 30.0
	newerAvg := 75.5

	mock := &mockCloudWatchAPI{
		fn: func(ctx context.Context, params *cloudwatch.GetMetricStatisticsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricStatisticsOutput, error) {
			return &cloudwatch.GetMetricStatisticsOutput{
				Datapoints: []types.Datapoint{
					{Timestamp: &older, Average: &olderAvg},
					{Timestamp: &newer, Average: &newerAvg},
				},
			}, nil
		},
	}
	client := newCloudWatchClientWithAPI(mock)

	metrics, err := client.GetInstanceMetrics(context.Background(), "i-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if metrics.CPUPercent != 75.5 {
		t.Errorf("CPUPercent = %f, want 75.5 (latest datapoint)", metrics.CPUPercent)
	}
	if metrics.MemoryPercent != -1 {
		t.Errorf("MemoryPercent = %f, want -1 (not available)", metrics.MemoryPercent)
	}
	if metrics.DiskPercent != -1 {
		t.Errorf("DiskPercent = %f, want -1 (not available)", metrics.DiskPercent)
	}
}

func TestGetInstanceMetrics_NoDatapoints(t *testing.T) {
	mock := &mockCloudWatchAPI{
		fn: func(ctx context.Context, params *cloudwatch.GetMetricStatisticsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricStatisticsOutput, error) {
			return &cloudwatch.GetMetricStatisticsOutput{Datapoints: []types.Datapoint{}}, nil
		},
	}
	client := newCloudWatchClientWithAPI(mock)

	metrics, err := client.GetInstanceMetrics(context.Background(), "i-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if metrics.CPUPercent != 0 {
		t.Errorf("CPUPercent = %f, want 0 when no datapoints", metrics.CPUPercent)
	}
}

func TestGetInstanceMetrics_Error(t *testing.T) {
	mock := &mockCloudWatchAPI{
		fn: func(ctx context.Context, params *cloudwatch.GetMetricStatisticsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricStatisticsOutput, error) {
			return nil, errors.New("access denied")
		},
	}
	client := newCloudWatchClientWithAPI(mock)

	_, err := client.GetInstanceMetrics(context.Background(), "i-123")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to get CloudWatch metrics") {
		t.Errorf("error = %q, want to contain 'failed to get CloudWatch metrics'", err)
	}
}

func TestGetInstanceMetrics_NilAverage(t *testing.T) {
	now := time.Now()
	mock := &mockCloudWatchAPI{
		fn: func(ctx context.Context, params *cloudwatch.GetMetricStatisticsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricStatisticsOutput, error) {
			return &cloudwatch.GetMetricStatisticsOutput{
				Datapoints: []types.Datapoint{
					{Timestamp: &now, Average: nil},
				},
			}, nil
		},
	}
	client := newCloudWatchClientWithAPI(mock)

	metrics, err := client.GetInstanceMetrics(context.Background(), "i-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if metrics.CPUPercent != 0 {
		t.Errorf("CPUPercent = %f, want 0 when Average is nil", metrics.CPUPercent)
	}
}
