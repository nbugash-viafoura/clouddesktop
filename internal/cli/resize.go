package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/nbugash-viafoura/clouddesktop/internal/config"
	"github.com/nbugash-viafoura/clouddesktop/internal/environment"
	"github.com/spf13/cobra"
)

// NewResizeInstanceCmd returns the resize-instance command which changes the instance type.
func NewResizeInstanceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resize-instance [name]",
		Short: "Resize cloud desktop instance type",
		Long:  "Changes an environment's EC2 instance type. Presents a selection of supported instance types, stops the instance if running, applies the change, then restarts. Without a name, it uses your only environment or your default.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResize(nameArg(args))
		},
	}
}

func runResize(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	s, err := openSession(ctx)
	if err != nil {
		return err
	}

	env, err := s.resolve(name, environment.OperationGeneral)
	if err != nil {
		return err
	}
	printTarget(env)

	var newInstanceType string
	instanceTypeOptions := make([]huh.Option[string], len(config.ValidInstanceTypes))
	for i, it := range config.ValidInstanceTypes {
		instanceTypeOptions[i] = huh.NewOption(it.Label, it.Value)
	}

	selectForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(fmt.Sprintf("Instance type for %s (current: %s)", env.Name, env.InstanceType)).
				Options(instanceTypeOptions...).
				Value(&newInstanceType),
		),
	)

	if err := selectForm.Run(); err != nil {
		return fmt.Errorf("resize cancelled: %w", err)
	}

	if env.InstanceType == newInstanceType {
		fmt.Printf("Instance is already type %s. Nothing to do.\n", newInstanceType)
		return nil
	}

	// Instance must be stopped to change type.
	if env.State == "running" {
		fmt.Printf("Stopping instance %s before resize...\n", env.InstanceID)
		if err := s.ec2.StopInstance(ctx, env.InstanceID); err != nil {
			return err
		}
		if err := s.ec2.WaitUntilStopped(ctx, env.InstanceID); err != nil {
			return fmt.Errorf("timed out waiting for instance to stop: %w", err)
		}
	} else if env.State != "stopped" {
		return fmt.Errorf("instance is in state '%s'. It must be stopped or running to resize", env.State)
	}

	fmt.Printf("Resizing from %s to %s...\n", env.InstanceType, newInstanceType)

	if err := s.ec2.ModifyInstanceType(ctx, env.InstanceID, newInstanceType); err != nil {
		return fmt.Errorf("resize failed: %w", err)
	}

	fmt.Printf("Starting instance with new type %s...\n", newInstanceType)
	if err := s.ec2.StartInstance(ctx, env.InstanceID); err != nil {
		return err
	}
	if err := s.ec2.WaitUntilRunning(ctx, env.InstanceID); err != nil {
		return fmt.Errorf("timed out waiting for instance to start: %w", err)
	}

	if err := s.syncSSHConfig(ctx); err != nil {
		return err
	}

	fmt.Printf("Resize complete. Instance is running as %s.\n", newInstanceType)
	return nil
}
