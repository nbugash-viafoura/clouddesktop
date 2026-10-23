package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	vfaws "github.com/nbugash-viafoura/clouddesktop/internal/aws"
	"github.com/nbugash-viafoura/clouddesktop/internal/config"
	"github.com/nbugash-viafoura/clouddesktop/internal/environment"
	"github.com/spf13/cobra"
)

// NewDestroyCmd returns the destroy command which permanently deletes one environment.
func NewDestroyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "destroy [name]",
		Short: "Permanently destroy cloud desktop",
		Long:  "Destroys one environment: its instance and all data on its disk. Your other environments and your personal S3 bucket are kept. The name is required when you have more than one environment. Without --confirm, it only shows what would be destroyed.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			confirm, _ := cmd.Flags().GetBool("confirm")
			return runDestroy(nameArg(args), confirm)
		},
	}

	cmd.Flags().Bool("confirm", false, "Confirm destructive operation")

	return cmd
}

func runDestroy(name string, confirmed bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	s, err := openSession(ctx)
	if err != nil {
		return err
	}

	env, err := s.resolve(name, environment.OperationDestroy)
	if err != nil {
		return err
	}
	printTarget(env)

	fmt.Printf("WARNING: This permanently destroys environment %q.\n", env.Name)
	fmt.Println("  All data on its disk (repos, Docker images, caches) will be DELETED.")
	fmt.Printf("  Your other environments are not affected: %s\n", joinOrNone(otherNames(s.envs, env.Name)))
	fmt.Printf("  Your personal bucket %s is kept. Delete it with 'clouddesktop s3-reset --confirm'.\n", bucketNameFor(ctx, s))
	fmt.Println()

	if !confirmed {
		return fmt.Errorf("re-run with --confirm to destroy environment %q", env.Name)
	}

	// List again right before destroying, so an environment created in the
	// meantime still counts and keeps the shared key pair alive.
	current, err := s.ec2.ListEnvironments(ctx, s.cfg.DeveloperName)
	if err != nil {
		return err
	}
	remaining := otherNames(current, env.Name)
	deleteKeyPair := len(remaining) == 0

	fmt.Println("Terminating instance...")
	if err := s.provisioner().Destroy(ctx, env.InstanceID, s.cfg.DeveloperName, deleteKeyPair); err != nil {
		return fmt.Errorf("destroy failed: %w", err)
	}

	if err := s.syncSSHConfig(ctx); err != nil {
		fmt.Printf("Warning: failed to clean up SSH config: %s\n", err)
	} else {
		fmt.Println("SSH config updated.")
	}

	if s.cfg.DefaultEnvironment == env.Name {
		if err := clearDefaultEnvironment(env.Name); err != nil {
			return err
		}
		if len(remaining) > 1 {
			fmt.Printf("%q was your default environment. Set a new one with 'clouddesktop use <name>'.\n", env.Name)
		}
	}

	fmt.Println()
	fmt.Printf("Environment %q destroyed.\n", env.Name)
	return nil
}

// clearDefaultEnvironment unsets the default if it still points at name. The
// tool never picks a new default on its own.
func clearDefaultEnvironment(name string) error {
	err := config.Update(func(cfg *config.Config) error {
		if cfg.DefaultEnvironment == name {
			cfg.DefaultEnvironment = ""
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to clear the default environment: %w", err)
	}
	return nil
}

// otherNames returns the sorted names of every environment except the given one.
func otherNames(envs []environment.Environment, except string) []string {
	var names []string
	for _, name := range environment.Names(envs) {
		if name != except {
			names = append(names, name)
		}
	}
	return names
}

// joinOrNone joins names with commas, or returns "none" for an empty list.
func joinOrNone(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

// bucketNameFor returns the developer's bucket name, falling back to the
// canonical name when the SSM parameter can't be read.
func bucketNameFor(ctx context.Context, s *session) string {
	name, err := s.ssm.GetDeveloperS3Bucket(ctx, s.cfg.DeveloperName)
	if err != nil || name == "" {
		return vfaws.BucketName(s.cfg.DeveloperName)
	}
	return name
}
