package cli

import (
	"github.com/nbugash-viafoura/clouddesktop/internal/version"
	"github.com/spf13/cobra"
)

// NewRootCmd returns the root cobra command for clouddesktop.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "clouddesktop",
		Short: "Cloud desktop management CLI for developers",
		Long:    "clouddesktop manages personal cloud development environments on AWS, providing commands to provision, control, and access EC2-based workstations.",
		Version: version.Version,
	}

	rootCmd.AddCommand(
		NewInitCmd(),
		NewUpCmd(),
		NewDownCmd(),
		NewStatusCmd(),
		NewSSHCmd(),
		NewResizeInstanceCmd(),
		NewResizeStorageCmd(),
		NewS3ResetCmd(),
		NewDestroyCmd(),
	)

	return rootCmd
}
