package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	vfaws "github.com/nbugash-viafoura/clouddesktop/internal/aws"
	"github.com/nbugash-viafoura/clouddesktop/internal/config"
	"github.com/nbugash-viafoura/clouddesktop/internal/terraform"
	"github.com/spf13/cobra"
)

// NewDestroyCmd returns the destroy command which permanently deletes the cloud desktop.
func NewDestroyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Permanently destroy cloud desktop",
		Long:  "Destroys the cloud desktop instance and all associated resources. This operation is irreversible and will delete all data on the instance.",
		RunE: func(cmd *cobra.Command, args []string) error {
			confirm, _ := cmd.Flags().GetBool("confirm")
			if !confirm {
				return errors.New("use --confirm to acknowledge this is destructive")
			}
			return runDestroy()
		},
	}

	cmd.Flags().Bool("confirm", false, "Confirm destructive operation")

	return cmd
}

func runDestroy() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.InstanceID == "" {
		fmt.Println("No instance provisioned. Nothing to destroy.")
		return cleanupConfig()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := vfaws.ValidateSession(ctx, cfg.AWSProfile, cfg.Region); err != nil {
		return err
	}

	fmt.Println("WARNING: This will permanently destroy your cloud desktop instance")
	fmt.Printf("  Instance ID:   %s\n", cfg.InstanceID)
	fmt.Printf("  Developer:     %s\n", cfg.DeveloperName)
	fmt.Println()
	fmt.Println("All data on the EBS volume (repos, Docker images, caches) will be DELETED.")
	fmt.Println()

	// Check instance state before destroying -- if it's already terminated,
	// just clean up Terraform state and config.
	ec2Client, err := vfaws.NewEC2Client(ctx, cfg.AWSProfile, cfg.Region)
	if err != nil {
		return err
	}

	info, err := ec2Client.DescribeInstance(ctx, cfg.InstanceID)
	if err != nil {
		fmt.Printf("Warning: could not describe instance: %s\n", err)
		fmt.Println("Proceeding with Terraform destroy anyway...")
	} else if info.State == "running" {
		fmt.Println("Stopping instance before destroying...")
		if err := ec2Client.StopInstance(ctx, cfg.InstanceID); err != nil {
			fmt.Printf("Warning: failed to stop instance: %s\n", err)
		} else {
			_ = ec2Client.WaitUntilStopped(ctx, cfg.InstanceID)
		}
	}

	fmt.Println("Running terraform destroy...")

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
		"instance_type":  cfg.InstanceType,
		"ssh_public_key": cfg.SSHPublicKey,
		"region":         cfg.Region,
	}
	if err := runner.Destroy(ctx, vars); err != nil {
		return fmt.Errorf("terraform destroy failed: %w", err)
	}

	if err := cleanupConfig(); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Cloud desktop destroyed. Run 'clouddesktop init' to create a new one.")
	return nil
}

// cleanupConfig removes the config file so clouddesktop init can be run again.
func cleanupConfig() error {
	configPath := config.ConfigPath()
	if configPath == "" {
		return nil
	}
	err := os.Remove(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove config file: %w", err)
	}
	return nil
}
