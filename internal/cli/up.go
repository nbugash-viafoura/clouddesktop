package cli

import (
	"context"
	"fmt"
	"time"

	vfaws "github.com/nbugash-viafoura/clouddesktop/internal/aws"
	"github.com/nbugash-viafoura/clouddesktop/internal/config"
	"github.com/nbugash-viafoura/clouddesktop/scripts"
	"github.com/spf13/cobra"
)

// NewUpCmd returns the up command which starts or provisions the cloud desktop.
func NewUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Start or provision cloud desktop",
		Long:  "Provisions a new cloud desktop instance if one doesn't exist, or starts an existing stopped instance. Developer identity is read from the config file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUp()
		},
	}
}

func runUp() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	if err := vfaws.ValidateSession(ctx, cfg.AWSProfile, cfg.Region); err != nil {
		return err
	}

	// If no instance ID exists in config, this is a first-time provision.
	if cfg.InstanceID == "" {
		return runProvision(ctx, cfg)
	}

	return runStart(ctx, cfg)
}

// runProvision handles first-time instance creation using the AWS SDK directly.
func runProvision(ctx context.Context, cfg *config.Config) error {
	fmt.Println("No existing instance found. Provisioning a new cloud desktop...")
	fmt.Println()

	ec2Client, err := vfaws.NewEC2Client(ctx, cfg.AWSProfile, cfg.Region)
	if err != nil {
		return err
	}

	ssmClient, err := vfaws.NewSSMClient(ctx, cfg.AWSProfile, cfg.Region)
	if err != nil {
		return err
	}

	provisioner := vfaws.NewProvisioner(ec2Client, ssmClient)

	fmt.Printf("Provisioning instance (type: %s)...\n", cfg.InstanceType)
	result, err := provisioner.Provision(ctx, vfaws.ProvisionParams{
		DeveloperName: cfg.DeveloperName,
		InstanceType:  cfg.InstanceType,
		SSHPublicKey:  cfg.SSHPublicKey,
		UserData:      scripts.BootstrapSystem,
	})
	if err != nil {
		return fmt.Errorf("provisioning failed: %w", err)
	}

	cfg.InstanceID = result.InstanceID
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save instance ID to config: %w", err)
	}

	if result.Recovered {
		fmt.Printf("Recovered existing instance %s.\n", result.InstanceID)
	} else {
		fmt.Printf("Instance %s created. Waiting for it to start...\n", result.InstanceID)
	}

	if err := ec2Client.WaitUntilRunning(ctx, result.InstanceID); err != nil {
		return fmt.Errorf("timed out waiting for instance to start: %w", err)
	}

	if err := writeSSHConfig(ctx, ec2Client, cfg); err != nil {
		return err
	}

	if !result.Recovered {
		fmt.Println()
		fmt.Println("Waiting for bootstrap to complete (this takes ~10 minutes on first provision)...")
		fmt.Println("You can check progress: clouddesktop ssh, then 'tail -f /var/log/bootstrap-system.log'")
	}

	info, err := ec2Client.DescribeInstance(ctx, result.InstanceID)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Cloud desktop is running.")
	printInstanceSummary(info, cfg)

	return nil
}

// runStart handles starting an already-provisioned but stopped instance.
func runStart(ctx context.Context, cfg *config.Config) error {
	ec2Client, err := vfaws.NewEC2Client(ctx, cfg.AWSProfile, cfg.Region)
	if err != nil {
		return err
	}

	info, err := ec2Client.DescribeInstance(ctx, cfg.InstanceID)
	if err != nil {
		return err
	}

	switch info.State {
	case "running":
		fmt.Println("Cloud desktop is already running.")
	case "stopped":
		fmt.Printf("Starting instance %s...\n", cfg.InstanceID)
		if err := ec2Client.StartInstance(ctx, cfg.InstanceID); err != nil {
			return err
		}
		if err := ec2Client.WaitUntilRunning(ctx, cfg.InstanceID); err != nil {
			return fmt.Errorf("timed out waiting for instance to start: %w", err)
		}
	case "pending":
		fmt.Println("Instance is already starting. Waiting...")
		if err := ec2Client.WaitUntilRunning(ctx, cfg.InstanceID); err != nil {
			return fmt.Errorf("timed out waiting for instance to start: %w", err)
		}
	case "stopping":
		fmt.Println("Instance is currently stopping. Wait for it to stop, then run 'clouddesktop up' again.")
		return nil
	case "terminated":
		fmt.Println("Instance has been terminated. Run 'clouddesktop destroy --confirm' to clean up, then 'clouddesktop init' to start fresh.")
		return nil
	default:
		return fmt.Errorf("instance is in unexpected state: %s", info.State)
	}

	// Always ensure SSH config is up to date when the instance is running.
	if err := writeSSHConfig(ctx, ec2Client, cfg); err != nil {
		return err
	}

	info, err = ec2Client.DescribeInstance(ctx, cfg.InstanceID)
	if err != nil {
		return err
	}
	fmt.Println("Cloud desktop is running.")
	printInstanceSummary(info, cfg)
	return nil
}

// writeSSHConfig updates ~/.ssh/config with the clouddesktop-managed block.
func writeSSHConfig(ctx context.Context, ec2Client *vfaws.EC2Client, cfg *config.Config) error {
	info, err := ec2Client.DescribeInstance(ctx, cfg.InstanceID)
	if err != nil {
		return fmt.Errorf("failed to get instance info for SSH config: %w", err)
	}

	entry := vfaws.SSHConfigEntry{
		HostAlias:    "clouddesktop",
		InstanceID:   info.InstanceID,
		User:         "ubuntu",
		IdentityFile: cfg.SSHKeyPath,
		AWSProfile:   cfg.AWSProfile,
		Region:       cfg.Region,
	}
	if err := vfaws.WriteSSHConfig(entry); err != nil {
		return fmt.Errorf("failed to update SSH config: %w", err)
	}

	fmt.Println("SSH config updated. Connect with: ssh clouddesktop")
	return nil
}

// printInstanceSummary prints a formatted summary of the instance.
func printInstanceSummary(info *vfaws.InstanceInfo, cfg *config.Config) {
	fmt.Println()
	fmt.Printf("  Instance ID:   %s\n", info.InstanceID)
	fmt.Printf("  State:         %s\n", info.State)
	fmt.Printf("  Type:          %s\n", info.InstanceType)
	if info.PrivateIP != "" {
		fmt.Printf("  Private IP:    %s\n", info.PrivateIP)
	}
	fmt.Printf("  SSH:           ssh clouddesktop\n")
	fmt.Printf("  Profile:       %s\n", cfg.AWSProfile)
}

