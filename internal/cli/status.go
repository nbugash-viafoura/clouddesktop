package cli

import (
	"context"
	"fmt"
	"time"

	vfaws "github.com/nbugash-viafoura/clouddesktop/internal/aws"
	"github.com/nbugash-viafoura/clouddesktop/internal/environment"
	"github.com/spf13/cobra"
)

// NewStatusCmd returns the status command which displays cloud desktop information.
func NewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [name]",
		Short: "Show cloud desktop status",
		Long:  "Displays the current state of an environment including instance state, type, IP address, and resource utilization metrics. Without a name, it uses your only environment or your default.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(nameArg(args))
		},
	}
}

func runStatus(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s, err := openSession(ctx)
	if err != nil {
		return err
	}

	env, err := s.resolve(name, environment.OperationGeneral)
	if err != nil {
		// Having nothing provisioned is a valid status, not a failure (exit 0, as before).
		if name == "" && environment.IsKind(err, environment.NoEnvironments) {
			fmt.Println("You have no environments.")
			fmt.Println("Run 'clouddesktop up' or 'clouddesktop create <name>' to provision one.")
			return nil
		}
		return err
	}
	printTarget(env)

	info, err := s.ec2.DescribeInstance(ctx, env.InstanceID)
	if err != nil {
		return err
	}

	cfg := s.cfg
	fmt.Println()
	fmt.Println("Cloud Desktop Status")
	fmt.Println("--------------------")
	fmt.Printf("  Environment:   %s\n", env.Name)
	fmt.Printf("  Default:       %s\n", yesNo(env.Name == cfg.DefaultEnvironment))
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
			metrics, err := cwClient.GetInstanceMetrics(ctx, env.InstanceID)
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
	switch info.State {
	case "running":
		fmt.Printf("Connect with: ssh %s\n", environment.SSHAlias(env.Name))
	case "stopped":
		fmt.Printf("Start with: clouddesktop up %s\n", env.Name)
	}

	return nil
}

// yesNo formats a boolean for display.
func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
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
