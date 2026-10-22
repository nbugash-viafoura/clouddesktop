package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nbugash-viafoura/clouddesktop/internal/environment"
	"github.com/spf13/cobra"
)

// NewS3ResetCmd returns the s3-reset command which deletes the developer's S3 bucket.
func NewS3ResetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "s3-reset",
		Short: "Delete and recreate the S3 bucket and mount",
		Long:  "Deletes the developer's S3 bucket (and all its contents) and removes the SSM parameter. The bucket is shared by all of your environments; each one gets a fresh bucket and mount on its next 'clouddesktop up'. Without --confirm, it only shows which environments would be affected.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			confirm, _ := cmd.Flags().GetBool("confirm")
			return runS3Reset(confirm)
		},
	}

	cmd.Flags().Bool("confirm", false, "Confirm destructive operation")

	return cmd
}

func runS3Reset(confirmed bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	s, err := openSession(ctx)
	if err != nil {
		return err
	}

	bucketName, err := s.ssm.GetDeveloperS3Bucket(ctx, s.cfg.DeveloperName)
	if err != nil {
		return err
	}

	if bucketName == "" {
		fmt.Println("No S3 bucket configured. Nothing to reset.")
		return nil
	}

	fmt.Printf("This deletes bucket %s and all its contents.\n", bucketName)
	fmt.Printf("Environments that use it: %s\n", describeEnvironments(s.envs))

	if !confirmed {
		return fmt.Errorf("re-run with --confirm to delete bucket %s", bucketName)
	}

	exists, err := s.s3.BucketExists(ctx, bucketName)
	if err != nil {
		return err
	}

	if exists {
		fmt.Println("Emptying bucket...")
		if err := s.s3.EmptyBucket(ctx, bucketName); err != nil {
			return fmt.Errorf("failed to empty bucket: %w", err)
		}

		if err := s.s3.DeleteBucket(ctx, bucketName); err != nil {
			return fmt.Errorf("failed to delete bucket: %w", err)
		}
	} else {
		fmt.Println("Bucket already deleted externally.")
	}

	if err := s.ssm.DeleteDeveloperS3Bucket(ctx, s.cfg.DeveloperName); err != nil {
		return fmt.Errorf("failed to remove SSM parameter: %w", err)
	}

	fmt.Println("S3 bucket and configuration removed.")
	if len(s.envs) > 0 {
		fmt.Println("Their mount will be repaired on their next 'clouddesktop up'.")
	} else {
		fmt.Println("Your next environment gets a fresh bucket.")
	}
	return nil
}

// describeEnvironments lists environments with their state, e.g. "main (running), build (stopped)".
func describeEnvironments(envs []environment.Environment) string {
	if len(envs) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(envs))
	for _, env := range envs {
		parts = append(parts, fmt.Sprintf("%s (%s)", env.Name, env.State))
	}
	return strings.Join(parts, ", ")
}
