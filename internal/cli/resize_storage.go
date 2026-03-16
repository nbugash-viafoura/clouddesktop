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

// NewResizeStorageCmd returns the resize-storage command which increases the root EBS volume size.
func NewResizeStorageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resize-storage",
		Short: "Increase root disk storage",
		Long:  "Increases the root EBS volume size. Presents a list of supported sizes larger than the current volume, applies the change online (no stop required), then extends the filesystem automatically.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResizeStorage()
		},
	}
}

func runResizeStorage() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.InstanceID == "" {
		return fmt.Errorf("no instance provisioned. Run 'clouddesktop up' first")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
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

	if info.State != "running" {
		return fmt.Errorf("instance is not running. Start your instance first with 'clouddesktop up'")
	}

	volumeID, currentSizeGB, err := ec2Client.GetRootVolumeInfo(ctx, cfg.InstanceID)
	if err != nil {
		return err
	}

	sizeOptions := []huh.Option[int]{
		huh.NewOption(fmt.Sprintf("%d GB (current — no change)", currentSizeGB), int(currentSizeGB)),
	}
	for _, size := range config.ValidStorageSizes {
		if size > int(currentSizeGB) {
			sizeOptions = append(sizeOptions, huh.NewOption(fmt.Sprintf("%d GB", size), size))
		}
	}

	if len(sizeOptions) == 1 {
		return fmt.Errorf("you've hit the limit of 2 TB — EBS volumes cannot be shrunk, and 2048 GB is the maximum supported size")
	}

	var newSizeGB int
	selectForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title(fmt.Sprintf("Root volume size (current: %d GB)", currentSizeGB)).
				Options(sizeOptions...).
				Value(&newSizeGB),
		),
	)

	if err := selectForm.Run(); err != nil {
		return fmt.Errorf("resize cancelled: %w", err)
	}

	if newSizeGB == int(currentSizeGB) {
		fmt.Printf("Root volume is already %d GB. No changes made.\n", currentSizeGB)
		return nil
	}

	fmt.Printf("Modifying root volume %s to %d GB...\n", volumeID, newSizeGB)
	if err := ec2Client.ModifyRootVolume(ctx, volumeID, int32(newSizeGB)); err != nil {
		return fmt.Errorf("failed to resize volume: %w", err)
	}

	fmt.Println("Waiting for volume resize to complete...")
	if err := ec2Client.WaitUntilVolumeResized(ctx, volumeID); err != nil {
		return fmt.Errorf("error waiting for volume resize: %w", err)
	}

	ssmClient, err := vfaws.NewSSMClient(ctx, cfg.AWSProfile, cfg.Region)
	if err != nil {
		return err
	}

	fmt.Println("Extending filesystem to use new storage...")
	commandID, err := ssmClient.RunFilesystemExtension(ctx, cfg.InstanceID)
	if err != nil {
		return fmt.Errorf("failed to run filesystem extension: %w", err)
	}

	fmt.Printf("Waiting for filesystem extension to complete (command: %s)...\n", commandID)
	if err := ssmClient.WaitUntilCommandComplete(ctx, cfg.InstanceID, commandID); err != nil {
		return fmt.Errorf("filesystem extension failed: %w", err)
	}

	cfg.RootVolumeSizeGB = newSizeGB
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Storage resize complete. Root volume is now %d GB.\n", newSizeGB)
	return nil
}
