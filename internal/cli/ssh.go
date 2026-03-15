package cli

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/nbugash-viafoura/clouddesktop/internal/config"
	"github.com/spf13/cobra"
)

// NewSSHCmd returns the ssh command which opens an SSH session to the cloud desktop.
func NewSSHCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ssh",
		Short: "Open SSH session to cloud desktop",
		Long:  "Opens an interactive SSH session to the cloud desktop via the SSM ProxyCommand configured in ~/.ssh/config.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSSH()
		},
	}
}

func runSSH() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.InstanceID == "" {
		return fmt.Errorf("no instance provisioned. Run 'clouddesktop up' first")
	}

	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh not found in PATH: %w", err)
	}

	// Replace the current process with ssh, using the clouddesktop-managed SSH config entry.
	// syscall.Exec replaces the process entirely -- no child process management needed.
	sshArgs := []string{"ssh", "clouddesktop"}
	if err := syscall.Exec(sshPath, sshArgs, os.Environ()); err != nil {
		return fmt.Errorf("failed to exec ssh: %w", err)
	}

	// Unreachable -- Exec replaces the process.
	return nil
}
