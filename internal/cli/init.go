package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nbugash-viafoura/clouddesktop/internal/config"
	"github.com/spf13/cobra"
)

// NewInitCmd returns the init command which initializes clouddesktop configuration.
func NewInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize clouddesktop configuration",
		Long:  "Creates the configuration file at ~/.clouddesktop/config.yaml by prompting for required AWS and developer settings.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit()
		},
	}
}

func runInit() error {
	existing, err := config.Load()
	if err != nil && !errors.Is(err, config.ErrConfigNotFound) {
		return fmt.Errorf("failed to check existing configuration: %w", err)
	}
	if existing != nil && existing.InstanceID != "" {
		return fmt.Errorf("clouddesktop is already initialized and an instance exists (ID: %s)\nRun 'clouddesktop destroy --confirm' before re-initializing", existing.InstanceID)
	}

	reader := bufio.NewReader(os.Stdin)
	cfg := &config.Config{}

	// Developer name
	developerName, err := prompt(reader, "Developer name (lowercase, used in resource naming)", "")
	if err != nil {
		return err
	}
	cfg.DeveloperName = developerName
	if cfg.DeveloperName == "" {
		return fmt.Errorf("developer name is required")
	}

	// AWS profile
	awsProfile, err := prompt(reader, "AWS profile", "test-developers")
	if err != nil {
		return err
	}
	cfg.AWSProfile = awsProfile

	// Region
	region, err := prompt(reader, "AWS region", "us-east-1")
	if err != nil {
		return err
	}
	cfg.Region = region

	// Instance type
	instanceType, err := prompt(reader, "Instance type", "m7i.xlarge")
	if err != nil {
		return err
	}
	cfg.InstanceType = instanceType

	// SSH public key path
	defaultKeyPath := defaultSSHKeyPath()
	keyPath, err := prompt(reader, "SSH public key path", defaultKeyPath)
	if err != nil {
		return err
	}
	keyPath = expandPath(keyPath)

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read SSH public key at %s: %w", keyPath, err)
	}
	cfg.SSHPublicKey = strings.TrimSpace(string(keyData))

	// Validate SSH key format (must be ed25519 or rsa)
	if !strings.HasPrefix(cfg.SSHPublicKey, "ssh-ed25519 ") && !strings.HasPrefix(cfg.SSHPublicKey, "ssh-rsa ") {
		return fmt.Errorf("SSH public key format is invalid. Expected ed25519 (ssh-ed25519) or RSA (ssh-rsa) format")
	}

	// Store the private key path (public key path minus .pub) for SSH config
	cfg.SSHKeyPath = strings.TrimSuffix(keyPath, ".pub")

	// S3 state bucket
	cfg.StateS3Bucket = "viafoura-clouddesktop-tfstate"

	// Shell config mirror prompt
	fmt.Println()
	fmt.Println("clouddesktop can copy your local ~/.zshrc (or ~/.bashrc) to the remote instance so your")
	fmt.Println("shell environment matches your local setup. This includes environment variables,")
	fmt.Println("aliases, and tool configuration (e.g. Docker, Testcontainers, fnm).")
	fmt.Println()
	mirror, err := prompt(reader, "Mirror local shell config to remote instance? [y/N]", "n")
	if err != nil {
		return err
	}
	if strings.ToLower(mirror) == "y" || strings.ToLower(mirror) == "yes" {
		shellPath, err := prompt(reader, "Path to shell config file", defaultShellConfigPath())
		if err != nil {
			return err
		}
		shellPath = expandPath(shellPath)
		if _, err := os.Stat(shellPath); err != nil {
			return fmt.Errorf("shell config file not found at %s: %w", shellPath, err)
		}
		cfg.ShellConfigPath = shellPath
	} else {
		fmt.Println()
		fmt.Println("Skipped. You can set up your shell config manually after SSHing into the instance,")
		fmt.Println("or run 'clouddesktop destroy --confirm' and re-run 'clouddesktop init' to redo this step.")
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println()
	fmt.Printf("Configuration saved to %s\n", config.ConfigPath())
	fmt.Println("Run 'clouddesktop up' to provision your cloud desktop.")

	return nil
}

// prompt displays a prompt and returns the user's input or the default value.
// Returns an error if there's an EOF or I/O error reading from stdin.
func prompt(reader *bufio.Reader, label, defaultVal string) (string, error) {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("%s: ", label)
	}
	input, err := reader.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			return "", fmt.Errorf("unexpected end of input reading %q (did you press Ctrl+D?)", label)
		}
		return "", fmt.Errorf("failed to read input for %q: %w", label, err)
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal, nil
	}
	return input, nil
}

// defaultSSHKeyPath returns the default path to an ed25519 public key.
func defaultSSHKeyPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.ssh/viafoura_dev.pub"
	}
	path := filepath.Join(home, ".ssh", "viafoura_dev.pub")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	// Fallback to default id_ed25519.pub
	path = filepath.Join(home, ".ssh", "id_ed25519.pub")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return filepath.Join(home, ".ssh", "viafoura_dev.pub")
}

// defaultShellConfigPath returns the path to the user's likely shell config.
func defaultShellConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.zshrc"
	}
	zshrc := filepath.Join(home, ".zshrc")
	if _, err := os.Stat(zshrc); err == nil {
		return zshrc
	}
	return filepath.Join(home, ".bashrc")
}

// expandPath expands ~ to the user's home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
