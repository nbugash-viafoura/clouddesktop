package cli

import (
	"context"
	"fmt"
	"time"

	vfaws "github.com/nbugash-viafoura/clouddesktop/internal/aws"
	"github.com/nbugash-viafoura/clouddesktop/internal/config"
	"github.com/spf13/cobra"
)

// NewDownCmd returns the down command which stops the cloud desktop.
func NewDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Stop cloud desktop",
		Long:  "Stops the running cloud desktop instance to avoid incurring compute costs. The instance and its data are preserved.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDown()
		},
	}
}

func runDown() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.InstanceID == "" {
		return fmt.Errorf("no instance provisioned. Run 'clouddesktop up' first")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
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

	switch info.State {
	case "stopped":
		fmt.Println("Cloud desktop is already stopped.")
		return nil
	case "stopping":
		fmt.Println("Cloud desktop is already stopping. Waiting...")
		if err := ec2Client.WaitUntilStopped(ctx, cfg.InstanceID); err != nil {
			return fmt.Errorf("timed out waiting for instance to stop: %w", err)
		}
		fmt.Println("Cloud desktop is stopped.")
		return nil
	case "running":
		fmt.Printf("Stopping instance %s...\n", cfg.InstanceID)
		if err := ec2Client.StopInstance(ctx, cfg.InstanceID); err != nil {
			return err
		}
		if err := ec2Client.WaitUntilStopped(ctx, cfg.InstanceID); err != nil {
			return fmt.Errorf("timed out waiting for instance to stop: %w", err)
		}
		fmt.Println("Cloud desktop is stopped. Data is preserved on EBS.")
		fmt.Println("Run 'clouddesktop up' to resume.")
		return nil
	case "terminated":
		return fmt.Errorf("instance has been terminated. Run 'clouddesktop destroy --confirm' to clean up, then 'clouddesktop init' to start fresh")
	default:
		return fmt.Errorf("instance is in state '%s' and cannot be stopped right now", info.State)
	}
}
