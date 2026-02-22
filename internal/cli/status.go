package cli

import (
	"context"
	"fmt"
	"time"

	vfaws "github.com/nbugash-viafoura/clouddesktop/internal/aws"
	"github.com/nbugash-viafoura/clouddesktop/internal/config"
	"github.com/spf13/cobra"
)

// NewStatusCmd returns the status command which displays cloud desktop information.
func NewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show cloud desktop status",
		Long:  "Displays the current state of the cloud desktop including instance state, type, IP address, and resource utilization metrics.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus()
		},
	}
}

func runStatus() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.InstanceID == "" {
		fmt.Println("No instance provisioned.")
		fmt.Println("Run 'clouddesktop up' to provision a cloud desktop.")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := vfaws.ValidateSession(ctx, cfg.AWSProfile, cfg.Region); err != nil {
		return err
	}

	ec2Client, err := vfaws.NewEC2Client(ctx, cfg.AWSProfile, cfg.Region)
	if err != nil {
		return err
	}

	info, err := ec2Client.DescribeInstance(ctx, cfg.InstanceID)
	if err != nil {
		return err
	}

	fmt.Println("Cloud Desktop Status")
	fmt.Println("--------------------")
	fmt.Printf("  Developer:     %s\n", cfg.DeveloperName)
	fmt.Printf("  Instance ID:   %s\n", info.InstanceID)
	fmt.Printf("  State:         %s\n", info.State)
	fmt.Printf("  Type:          %s\n", info.InstanceType)
	if info.PrivateIP != "" {
		fmt.Printf("  Private IP:    %s\n", info.PrivateIP)
	}
	if info.LaunchTime != nil {
		fmt.Printf("  Launch Time:   %s\n", info.LaunchTime.Format(time.RFC3339))
		if info.State == "running" {
			uptime := time.Since(*info.LaunchTime).Round(time.Minute)
			fmt.Printf("  Uptime:        %s\n", formatDuration(uptime))
		}
	}
	fmt.Printf("  AWS Profile:   %s\n", cfg.AWSProfile)
	fmt.Printf("  Region:        %s\n", cfg.Region)

	// Fetch CloudWatch metrics if the instance is running.
	if info.State == "running" {
		cwClient, err := vfaws.NewCloudWatchClient(ctx, cfg.AWSProfile, cfg.Region)
		if err == nil {
			metrics, err := cwClient.GetInstanceMetrics(ctx, cfg.InstanceID)
			if err == nil {
				fmt.Println()
				fmt.Println("Metrics (5-min avg)")
				fmt.Println("-------------------")
				fmt.Printf("  CPU:           %.1f%%\n", metrics.CPUPercent)
				if metrics.MemoryPercent >= 0 {
					fmt.Printf("  Memory:        %.1f%%\n", metrics.MemoryPercent)
				} else {
					fmt.Printf("  Memory:        N/A (CloudWatch agent not yet reporting)\n")
				}
				if metrics.DiskPercent >= 0 {
					fmt.Printf("  Disk:          %.1f%%\n", metrics.DiskPercent)
				} else {
					fmt.Printf("  Disk:          N/A (CloudWatch agent not yet reporting)\n")
				}
			}
		}
	}

	fmt.Println()
	if info.State == "running" {
		fmt.Println("Connect with: ssh clouddesktop")
	} else if info.State == "stopped" {
		fmt.Println("Start with: clouddesktop up")
	}

	return nil
}

// formatDuration formats a duration into a human-readable string like "2h 30m".
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
