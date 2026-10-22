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

// NewResizeStorageCmd returns the resize-storage command which increases the root EBS volume size.
func NewResizeStorageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resize-storage [name]",
		Short: "Increase root disk storage",
		Long:  "Increases an environment's root EBS volume size. Presents a list of supported sizes larger than the current volume, applies the change online (no stop required), then extends the filesystem automatically. Without a name, it uses your only environment or your default.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResizeStorage(nameArg(args))
		},
	}
}

func runResizeStorage(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
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

	if env.State != "running" {
		return fmt.Errorf("instance is not running. Start it first with 'clouddesktop up %s'", env.Name)
	}

	volumeID, currentSizeGB, err := s.ec2.GetRootVolumeInfo(ctx, env.InstanceID)
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
				Title(fmt.Sprintf("Root volume size for %s (current: %d GB)", env.Name, currentSizeGB)).
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
	if err := s.ec2.ModifyRootVolume(ctx, volumeID, int32(newSizeGB)); err != nil {
		return fmt.Errorf("failed to resize volume: %w", err)
	}

	fmt.Println("Waiting for volume resize to complete...")
	if err := s.ec2.WaitUntilVolumeResized(ctx, volumeID); err != nil {
		return fmt.Errorf("error waiting for volume resize: %w", err)
	}

	fmt.Println("Extending filesystem to use new storage...")
	commandID, err := s.ssm.RunFilesystemExtension(ctx, env.InstanceID)
	if err != nil {
		return fmt.Errorf("failed to run filesystem extension: %w", err)
	}

	fmt.Printf("Waiting for filesystem extension to complete (command: %s)...\n", commandID)
	if err := s.ssm.WaitUntilCommandComplete(ctx, env.InstanceID, commandID); err != nil {
		return fmt.Errorf("filesystem extension failed: %w", err)
	}

	fmt.Printf("Storage resize complete. Root volume is now %d GB.\n", newSizeGB)
	return nil
}
