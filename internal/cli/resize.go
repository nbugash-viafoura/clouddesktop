package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/huh"
	vfaws "github.com/nbugash-viafoura/clouddesktop/internal/aws"
	"github.com/nbugash-viafoura/clouddesktop/internal/config"
	"github.com/spf13/cobra"
)

// NewResizeInstanceCmd returns the resize-instance command which changes the instance type.
func NewResizeInstanceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resize-instance",
		Short: "Resize cloud desktop instance type",
		Long:  "Changes the EC2 instance type. Presents a selection of supported instance types, stops the instance if running, applies the change, then restarts.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResize()
		},
	}
}

func runResize() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.InstanceID == "" {
		return fmt.Errorf("no instance provisioned. Run 'clouddesktop up' first")
	}

	var newInstanceType string
	instanceTypeOptions := make([]huh.Option[string], len(config.ValidInstanceTypes))
	for i, it := range config.ValidInstanceTypes {
		instanceTypeOptions[i] = huh.NewOption(it.Label, it.Value)
	}

	selectForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(fmt.Sprintf("Instance type (current: %s)", cfg.InstanceType)).
				Options(instanceTypeOptions...).
				Value(&newInstanceType),
		),
	)

	if err := selectForm.Run(); err != nil {
		return fmt.Errorf("resize cancelled: %w", err)
	}

	if cfg.InstanceType == newInstanceType {
		fmt.Printf("Instance is already type %s. Nothing to do.\n", newInstanceType)
		return nil
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

	if err := ec2Client.ModifyInstanceType(ctx, cfg.InstanceID, newInstanceType); err != nil {
		return fmt.Errorf("resize failed: %w", err)
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
