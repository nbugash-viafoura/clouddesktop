package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/nbugash-viafoura/clouddesktop/internal/config"
	"github.com/nbugash-viafoura/clouddesktop/internal/environment"
	"github.com/spf13/cobra"
)

// NewUseCmd returns the use command which sets the default environment.
func NewUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Set your default cloud desktop",
		Long:  "Sets the environment that commands act on when you have more than one environment and don't name one. Destroy never uses the default.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUse(args[0])
		},
	}
}

func runUse(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s, err := openSession(ctx)
	if err != nil {
		return err
	}

	env, err := s.resolve(name, environment.OperationGeneral)
	if err != nil {
		return err
	}

	if err := config.Update(func(cfg *config.Config) error {
		cfg.DefaultEnvironment = env.Name
		return nil
	}); err != nil {
		return fmt.Errorf("failed to set the default environment: %w", err)
	}

	fmt.Printf("Default environment: %s\n", env.Name)
	return nil
}
