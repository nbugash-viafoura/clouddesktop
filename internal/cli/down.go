package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	vfaws "github.com/nbugash-viafoura/clouddesktop/internal/aws"
	"github.com/nbugash-viafoura/clouddesktop/internal/environment"
	"github.com/spf13/cobra"
)

// NewDownCmd returns the down command which stops the cloud desktop.
func NewDownCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down [name]",
		Short: "Stop cloud desktop",
		Long:  "Stops an environment to avoid incurring compute costs. The instance and its data are preserved. Without a name, it uses your only environment or your default. With --all, it stops every running environment you own.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			all, _ := cmd.Flags().GetBool("all")
			if all {
				if len(args) > 0 {
					return errors.New("--all cannot be combined with an environment name")
				}
				return runDownAll()
			}
			return runDown(nameArg(args))
		},
	}

	cmd.Flags().Bool("all", false, "Stop every running environment")

	return cmd
}

func runDown(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
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

	switch env.State {
	case "stopped":
		fmt.Println("Cloud desktop is already stopped.")
		return nil
	case "stopping":
		fmt.Println("Cloud desktop is already stopping. Waiting...")
		if err := s.ec2.WaitUntilStopped(ctx, env.InstanceID); err != nil {
			return fmt.Errorf("timed out waiting for instance to stop: %w", err)
		}
		fmt.Println("Cloud desktop is stopped.")
		return nil
	case "running":
		fmt.Printf("Stopping instance %s...\n", env.InstanceID)
		if err := s.ec2.StopInstance(ctx, env.InstanceID); err != nil {
			return err
		}
		if err := s.ec2.WaitUntilStopped(ctx, env.InstanceID); err != nil {
			return fmt.Errorf("timed out waiting for instance to stop: %w", err)
		}
		fmt.Println("Cloud desktop is stopped. Data is preserved on EBS.")
		fmt.Printf("Run 'clouddesktop up %s' to resume.\n", env.Name)
		return nil
	default:
		return fmt.Errorf("instance is in state '%s' and cannot be stopped right now", env.State)
	}
}

func runDownAll() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	s, err := openSession(ctx)
	if err != nil {
		return err
	}

	report, err := stopAll(ctx, s.ec2, s.envs)
	fmt.Print(report)
	return err
}

// stopAll stops every running or pending environment. It issues every stop
// before waiting on any, keeps going past individual failures, and returns one
// report line per environment plus an error naming the environments that failed.
func stopAll(ctx context.Context, ec2 vfaws.EC2Service, envs []environment.Environment) (string, error) {
	var targets []environment.Environment
	for _, env := range envs {
		if env.State == "running" || env.State == "pending" {
			targets = append(targets, env)
		}
	}
	if len(targets) == 0 {
		return "No running environments.\n", nil
	}

	failures := make(map[string]error)
	for _, env := range targets {
		if err := ec2.StopInstance(ctx, env.InstanceID); err != nil {
			failures[env.Name] = err
		}
	}
	for _, env := range targets {
		if failures[env.Name] != nil {
			continue
		}
		if err := ec2.WaitUntilStopped(ctx, env.InstanceID); err != nil {
			failures[env.Name] = err
		}
	}

	var report strings.Builder
	var failed []string
	for _, env := range envs {
		switch {
		case failures[env.Name] != nil:
			fmt.Fprintf(&report, "  %s: failed: %s\n", env.Name, failures[env.Name])
			failed = append(failed, env.Name)
		case env.State == "running" || env.State == "pending":
			fmt.Fprintf(&report, "  %s: stopped\n", env.Name)
		default:
			fmt.Fprintf(&report, "  %s: already %s\n", env.Name, env.State)
		}
	}

	if len(failed) > 0 {
		return report.String(), fmt.Errorf("failed to stop: %s", strings.Join(failed, ", "))
	}
	return report.String(), nil
}
