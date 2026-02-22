package cli

import (
	"context"
	"fmt"
	"time"

	vfaws "github.com/nbugash-viafoura/clouddesktop/internal/aws"
	"github.com/nbugash-viafoura/clouddesktop/internal/config"
	"github.com/nbugash-viafoura/clouddesktop/internal/terraform"
	"github.com/spf13/cobra"
)

// NewResizeCmd returns the resize command which changes the instance type.
func NewResizeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resize <instance-type>",
		Short: "Resize cloud desktop instance type",
		Long:  "Changes the EC2 instance type (e.g., m7i.2xlarge). Stops the instance if running, applies the change via Terraform, then restarts.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResize(args[0])
		},
	}
}

func runResize(newInstanceType string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.InstanceID == "" {
		return fmt.Errorf("no instance provisioned. Run 'clouddesktop up' first")
	}

	if cfg.InstanceType == newInstanceType {
		return fmt.Errorf("instance is already type %s", newInstanceType)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
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

	// Instance must be stopped to change type.
	if info.State == "running" {
		fmt.Printf("Stopping instance %s before resize...\n", cfg.InstanceID)
		if err := ec2Client.StopInstance(ctx, cfg.InstanceID); err != nil {
			return err
		}
		if err := ec2Client.WaitUntilStopped(ctx, cfg.InstanceID); err != nil {
			return fmt.Errorf("timed out waiting for instance to stop: %w", err)
		}
	} else if info.State != "stopped" {
		return fmt.Errorf("instance is in state '%s'. It must be stopped or running to resize", info.State)
	}

	fmt.Printf("Resizing from %s to %s...\n", cfg.InstanceType, newInstanceType)

	tfDir, err := terraformInstanceDir()
	if err != nil {
		return err
	}

	backendKey := terraform.S3BackendKey(cfg.DeveloperName)
	runner := terraform.NewRunner(tfDir, cfg.AWSProfile, cfg.Region, backendKey)

	if err := runner.Init(ctx); err != nil {
		return fmt.Errorf("terraform init failed: %w", err)
	}

	vars := map[string]string{
		"developer_name": cfg.DeveloperName,
		"instance_type":  newInstanceType,
		"ssh_public_key": cfg.SSHPublicKey,
		"region":         cfg.Region,
	}
	if err := runner.Apply(ctx, vars); err != nil {
		return fmt.Errorf("terraform apply failed: %w", err)
	}

	// Update config with new instance type.
	cfg.InstanceType = newInstanceType
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Starting instance with new type %s...\n", newInstanceType)
	if err := ec2Client.StartInstance(ctx, cfg.InstanceID); err != nil {
		return err
	}
	if err := ec2Client.WaitUntilRunning(ctx, cfg.InstanceID); err != nil {
		return fmt.Errorf("timed out waiting for instance to start: %w", err)
	}

	if err := writeSSHConfig(ctx, ec2Client, cfg); err != nil {
		return err
	}

	fmt.Printf("Resize complete. Instance is running as %s.\n", newInstanceType)
	return nil
}
