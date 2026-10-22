package cli

import (
	"context"
	"fmt"
	"time"

	vfaws "github.com/nbugash-viafoura/clouddesktop/internal/aws"
	"github.com/nbugash-viafoura/clouddesktop/internal/config"
	"github.com/nbugash-viafoura/clouddesktop/internal/environment"
	"github.com/spf13/cobra"
)

// NewUpCmd returns the up command which starts or provisions the cloud desktop.
func NewUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up [name]",
		Short: "Start or provision cloud desktop",
		Long:  "Starts an environment if it is stopped, refreshes its SSH config entry, and makes sure your personal S3 bucket is mounted on it. Without a name, it uses your only environment or your default. If you have no environments yet, it provisions one named \"main\".",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUp(nameArg(args))
		},
	}
}

func runUp(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	s, err := openSession(ctx)
	if err != nil {
		return err
	}

	env, err := s.resolve(name, environment.OperationGeneral)
	if err != nil {
		// With no environments and no name, keep today's first-run behavior.
		if name == "" && environment.IsKind(err, environment.NoEnvironments) {
			fmt.Println("No existing instance found. Provisioning a new cloud desktop...")
			fmt.Println()
			return provisionEnvironment(ctx, s, environment.LegacyName, s.cfg.InstanceType, defaultStorageSizeGB)
		}
		return err
	}

	return runStart(ctx, s, env)
}

// runStart starts an already-provisioned environment if needed, then refreshes
// its SSH config entry and S3 mount.
func runStart(ctx context.Context, s *session, env environment.Environment) error {
	printTarget(env)

	switch env.State {
	case "running":
		fmt.Println("Cloud desktop is already running.")
	case "stopped":
		fmt.Printf("Starting instance %s...\n", env.InstanceID)
		if err := s.ec2.StartInstance(ctx, env.InstanceID); err != nil {
			return err
		}
		if err := s.ec2.WaitUntilRunning(ctx, env.InstanceID); err != nil {
			return fmt.Errorf("timed out waiting for instance to start: %w", err)
		}
	case "pending":
		fmt.Println("Instance is already starting. Waiting...")
		if err := s.ec2.WaitUntilRunning(ctx, env.InstanceID); err != nil {
			return fmt.Errorf("timed out waiting for instance to start: %w", err)
		}
	case "stopping":
		fmt.Printf("Instance is currently stopping. Wait for it to stop, then run 'clouddesktop up %s' again.\n", env.Name)
		return nil
	default:
		return fmt.Errorf("instance is in unexpected state: %s", env.State)
	}

	// Always ensure SSH config is up to date when the instance is running.
	if err := s.syncSSHConfig(ctx); err != nil {
		return err
	}

	ensureS3Mount(ctx, s, env.Name, env.InstanceID)

	info, err := s.ec2.DescribeInstance(ctx, env.InstanceID)
	if err != nil {
		return err
	}
	fmt.Println("Cloud desktop is running.")
	printInstanceSummary(env.Name, info, s.cfg)
	return nil
}

// ensureS3Mount makes sure the developer's bucket is mounted on the instance.
// A failure is reported as a warning: the environment itself is usable.
func ensureS3Mount(ctx context.Context, s *session, envName, instanceID string) {
	fmt.Printf("Setting up S3 mount on %s...\n", envName)
	if err := s.provisioner().EnsureS3Mount(ctx, instanceID, s.cfg.DeveloperName); err != nil {
		fmt.Printf("Warning: S3 mount setup failed: %s\n", err)
		fmt.Printf("  You can retry with 'clouddesktop up %s' once the instance is ready.\n", envName)
	}
}

// printInstanceSummary prints a formatted summary of the instance.
func printInstanceSummary(envName string, info *vfaws.InstanceInfo, cfg *config.Config) {
	fmt.Println()
	fmt.Printf("  Environment:   %s\n", envName)
	fmt.Printf("  Instance ID:   %s\n", info.InstanceID)
	fmt.Printf("  State:         %s\n", info.State)
	fmt.Printf("  Type:          %s\n", info.InstanceType)
	if info.PrivateIP != "" {
		fmt.Printf("  Private IP:    %s\n", info.PrivateIP)
	}
	fmt.Printf("  SSH:           ssh %s\n", environment.SSHAlias(envName))
	fmt.Printf("  Profile:       %s\n", cfg.AWSProfile)
}
