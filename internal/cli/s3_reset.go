package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	vfaws "github.com/nbugash-viafoura/clouddesktop/internal/aws"
	"github.com/nbugash-viafoura/clouddesktop/internal/config"
	"github.com/spf13/cobra"
)

// NewS3ResetCmd returns the s3-reset command which deletes and recreates the S3 mount.
func NewS3ResetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "s3-reset",
		Short: "Delete and recreate the S3 bucket and mount",
		Long:  "Deletes the developer's S3 bucket (and all its contents), removes the SSM parameter, and recreates everything from scratch. The next 'clouddesktop up' will set up a fresh mount.",
		RunE: func(cmd *cobra.Command, args []string) error {
			confirm, _ := cmd.Flags().GetBool("confirm")
			if !confirm {
				return errors.New("use --confirm to acknowledge this deletes all S3 bucket contents")
			}
			return runS3Reset()
		},
	}

	cmd.Flags().Bool("confirm", false, "Confirm destructive operation")

	return cmd
}

func runS3Reset() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := vfaws.ValidateSession(ctx, cfg.AWSProfile, cfg.Region); err != nil {
		return err
	}

	ssmClient, err := vfaws.NewSSMClient(ctx, cfg.AWSProfile, cfg.Region)
	if err != nil {
		return err
	}

	s3Client, err := vfaws.NewS3Client(ctx, cfg.AWSProfile, cfg.Region)
	if err != nil {
		return err
	}

	bucketName, err := ssmClient.GetDeveloperS3Bucket(ctx, cfg.DeveloperName)
	if err != nil {
		return err
	}

	if bucketName == "" {
		fmt.Println("No S3 bucket configured. Nothing to reset.")
		return nil
	}

	fmt.Printf("Deleting S3 bucket: %s\n", bucketName)

	exists, err := s3Client.BucketExists(ctx, bucketName)
	if err != nil {
		return err
	}

	if exists {
		fmt.Println("Emptying bucket...")
		if err := s3Client.EmptyBucket(ctx, bucketName); err != nil {
			return fmt.Errorf("failed to empty bucket: %w", err)
		}

		if err := s3Client.DeleteBucket(ctx, bucketName); err != nil {
			return fmt.Errorf("failed to delete bucket: %w", err)
		}
	} else {
		fmt.Println("Bucket already deleted externally.")
	}

	if err := ssmClient.DeleteDeveloperS3Bucket(ctx, cfg.DeveloperName); err != nil {
		return fmt.Errorf("failed to remove SSM parameter: %w", err)
	}

	fmt.Println("S3 bucket and configuration removed.")
	fmt.Println("Run 'clouddesktop up' to create a fresh bucket and mount.")
	return nil
}
