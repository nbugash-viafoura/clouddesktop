package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/nbugash-viafoura/clouddesktop/internal/environment"
	"github.com/spf13/cobra"
)

// NewSSHCmd returns the ssh command which opens an SSH session to the cloud desktop.
func NewSSHCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ssh [name]",
		Short: "Open SSH session to cloud desktop",
		Long:  "Opens an interactive SSH session to an environment via the SSM ProxyCommand configured in ~/.ssh/config. Without a name, it uses your only environment or your default.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSSH(nameArg(args))
		},
	}
}

func runSSH(name string) error {
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
	printTarget(env)

	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh not found in PATH: %w", err)
	}

	// Replace the current process with ssh, using the environment's managed SSH config entry.
	// syscall.Exec replaces the process entirely -- no child process management needed.
	sshArgs := []string{"ssh", environment.SSHAlias(env.Name)}
	if err := syscall.Exec(sshPath, sshArgs, os.Environ()); err != nil {
		return fmt.Errorf("failed to exec ssh: %w", err)
	}

	// Unreachable -- Exec replaces the process.
	return nil
}
