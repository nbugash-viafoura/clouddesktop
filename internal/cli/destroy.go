package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	vfaws "github.com/nbugash-viafoura/clouddesktop/internal/aws"
	"github.com/nbugash-viafoura/clouddesktop/internal/config"
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

	ec2Client, err := vfaws.NewEC2Client(ctx, cfg.AWSProfile, cfg.Region)
	if err != nil {
		return err
	}

	ssmClient, err := vfaws.NewSSMClient(ctx, cfg.AWSProfile, cfg.Region)
	if err != nil {
		return err
	}

	provisioner := vfaws.NewProvisioner(ec2Client, ssmClient)

	fmt.Println("Terminating instance and cleaning up resources...")
	if err := provisioner.Destroy(ctx, cfg.InstanceID, cfg.DeveloperName); err != nil {
		return fmt.Errorf("destroy failed: %w", err)
	}

	if err := vfaws.RemoveSSHConfig(); err != nil {
		fmt.Printf("Warning: failed to clean up SSH config: %s\n", err)
	} else {
		fmt.Println("SSH config cleaned up.")
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
