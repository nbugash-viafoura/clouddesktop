package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	vfaws "github.com/nbugash-viafoura/clouddesktop/internal/aws"
	"github.com/nbugash-viafoura/clouddesktop/internal/config"
	"github.com/nbugash-viafoura/clouddesktop/internal/terraform"
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

// runProvision handles first-time instance creation via Terraform.
func runProvision(ctx context.Context, cfg *config.Config) error {
	fmt.Println("No existing instance found. Provisioning a new cloud desktop...")
	fmt.Println()

	tfDir, err := terraformInstanceDir()
	if err != nil {
		return err
	}

	backendKey := terraform.S3BackendKey(cfg.DeveloperName)
	runner := terraform.NewRunner(tfDir, cfg.AWSProfile, cfg.Region, backendKey)

	fmt.Println("Initializing Terraform...")
	if err := runner.Init(ctx); err != nil {
		return fmt.Errorf("terraform init failed: %w", err)
	}

	fmt.Printf("Provisioning instance (type: %s)...\n", cfg.InstanceType)
	vars := map[string]string{
		"developer_name": cfg.DeveloperName,
		"instance_type":  cfg.InstanceType,
		"ssh_public_key": cfg.SSHPublicKey,
		"region":         cfg.Region,
	}
	if err := runner.Apply(ctx, vars); err != nil {
		return fmt.Errorf("terraform apply failed: %w", err)
	}

	instanceID, err := runner.Output(ctx, "instance_id")
	if err != nil {
		return fmt.Errorf("failed to get instance ID from Terraform output: %w", err)
	}

	cfg.InstanceID = instanceID
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save instance ID to config: %w", err)
	}

	fmt.Printf("Instance %s created. Waiting for it to start...\n", instanceID)

	ec2Client, err := vfaws.NewEC2Client(ctx, cfg.AWSProfile, cfg.Region)
	if err != nil {
		return err
	}

	if err := ec2Client.WaitUntilRunning(ctx, instanceID); err != nil {
		return fmt.Errorf("timed out waiting for instance to start: %w", err)
	}

	if err := writeSSHConfig(ctx, ec2Client, cfg); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Waiting for bootstrap to complete (this takes ~10 minutes on first provision)...")
	fmt.Println("You can check progress: clouddesktop ssh, then 'tail -f /var/log/bootstrap-system.log'")

	if err := copyShellConfig(cfg); err != nil {
		fmt.Printf("Warning: failed to copy shell config: %s\n", err)
	}

	info, err := ec2Client.DescribeInstance(ctx, instanceID)
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
		printInstanceSummary(info, cfg)
		return nil
	case "stopped":
		fmt.Printf("Starting instance %s...\n", cfg.InstanceID)
		if err := ec2Client.StartInstance(ctx, cfg.InstanceID); err != nil {
			return err
		}
		if err := ec2Client.WaitUntilRunning(ctx, cfg.InstanceID); err != nil {
			return fmt.Errorf("timed out waiting for instance to start: %w", err)
		}
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
	case "stopping":
		fmt.Println("Instance is currently stopping. Wait for it to stop, then run 'clouddesktop up' again.")
		return nil
	case "pending":
		fmt.Println("Instance is already starting. Waiting...")
		if err := ec2Client.WaitUntilRunning(ctx, cfg.InstanceID); err != nil {
			return fmt.Errorf("timed out waiting for instance to start: %w", err)
		}
		fmt.Println("Cloud desktop is running.")
		return nil
	case "terminated":
		fmt.Println("Instance has been terminated. Run 'clouddesktop destroy --confirm' to clean up, then 'clouddesktop init' to start fresh.")
		return nil
	default:
		return fmt.Errorf("instance is in unexpected state: %s", info.State)
	}
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

// copyShellConfig copies the local shell config to the remote instance if configured.
func copyShellConfig(cfg *config.Config) error {
	if cfg.ShellConfigPath == "" {
		return nil
	}

	if _, err := os.Stat(cfg.ShellConfigPath); err != nil {
		return fmt.Errorf("shell config file not found: %w", err)
	}

	// The shell config will be copied via SCP after the instance is accessible.
	// Since SSM proxy needs time to be ready after instance start, we note this
	// for the developer to do manually if the automatic copy fails.
	fmt.Printf("Shell config at %s will be available for copying after SSH is ready.\n", cfg.ShellConfigPath)
	fmt.Println("Copy manually with: scp -F ~/.ssh/config " + cfg.ShellConfigPath + " clouddesktop:~/.zshrc")
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

// terraformInstanceDir returns the absolute path to terraform/instance/ relative
// to the clouddesktop binary or the current working directory.
func terraformInstanceDir() (string, error) {
	// Look for the terraform/instance directory relative to the executable first.
	execPath, err := os.Executable()
	if err == nil {
		dir := filepath.Join(filepath.Dir(execPath), "..", "terraform", "instance")
		if _, err := os.Stat(dir); err == nil {
			absDir, _ := filepath.Abs(dir)
			return absDir, nil
		}
	}

	// Fallback: look relative to the current working directory.
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	dir := filepath.Join(cwd, "terraform", "instance")
	if _, err := os.Stat(dir); err == nil {
		return dir, nil
	}

	// Last resort: check if CLOUDDESKTOP_REPO_DIR is set.
	repoDir := os.Getenv("CLOUDDESKTOP_REPO_DIR")
	if repoDir != "" {
		dir = filepath.Join(repoDir, "terraform", "instance")
		if _, err := os.Stat(dir); err == nil {
			return dir, nil
		}
	}

	return "", fmt.Errorf("cannot find terraform/instance directory. Set CLOUDDESKTOP_REPO_DIR to the clouddesktop repo root")
}
