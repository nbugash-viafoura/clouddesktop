package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
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

	cfg := &config.Config{}

	// Developer name (text input)
	developerName := ""

	nameForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Developer name (lowercase, used in resource naming)").
				Value(&developerName).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("developer name is required")
					}
					return nil
				}),
		),
	)

	if err := nameForm.Run(); err != nil {
		return fmt.Errorf("init cancelled: %w", err)
	}

	cfg.DeveloperName = strings.TrimSpace(developerName)
	cfg.Region = "us-east-1"

	// AWS profile (select from available profiles)
	profiles, err := findAWSProfiles()
	if err != nil || len(profiles) == 0 {
		return fmt.Errorf("no AWS profiles found - run 'aws configure' to set up a profile first")
	}

	awsProfile := "test-terraform"
	profileOptions := make([]huh.Option[string], len(profiles))
	for i, p := range profiles {
		profileOptions[i] = huh.NewOption(p, p)
	}

	profileForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("AWS profile").
				Options(profileOptions...).
				Value(&awsProfile),
		),
	)

	if err := profileForm.Run(); err != nil {
		return fmt.Errorf("init cancelled: %w", err)
	}

	cfg.AWSProfile = awsProfile

	// Instance type (select), defaulting to m7i.xlarge.
	instanceType := "m7i.xlarge"
	instanceTypeOptions := make([]huh.Option[string], len(config.ValidInstanceTypes))
	for i, it := range config.ValidInstanceTypes {
		instanceTypeOptions[i] = huh.NewOption(it.Label, it.Value)
	}

	selectForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Instance type").
				Options(instanceTypeOptions...).
				Value(&instanceType),
		),
	)

	if err := selectForm.Run(); err != nil {
		return fmt.Errorf("init cancelled: %w", err)
	}

	cfg.InstanceType = instanceType

	// SSH public key (select from ~/.ssh/*.pub)
	sshKeys, err := findSSHPublicKeys()
	if err != nil {
		return fmt.Errorf("failed to scan SSH keys: %w", err)
	}

	var keyPath string
	if len(sshKeys) == 0 {
		return fmt.Errorf("no SSH public keys found in ~/.ssh/ - generate one with: ssh-keygen -t ed25519")
	}

	sshKeyOptions := make([]huh.Option[string], len(sshKeys))
	for i, key := range sshKeys {
		sshKeyOptions[i] = huh.NewOption(shortenPath(key), key)
	}

	keyForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("SSH public key").
				Options(sshKeyOptions...).
				Value(&keyPath),
		),
	)

	if err := keyForm.Run(); err != nil {
		return fmt.Errorf("init cancelled: %w", err)
	}

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read SSH public key at %s: %w", keyPath, err)
	}
	cfg.SSHPublicKey = strings.TrimSpace(string(keyData))

	if !strings.HasPrefix(cfg.SSHPublicKey, "ssh-ed25519 ") && !strings.HasPrefix(cfg.SSHPublicKey, "ssh-rsa ") {
		return fmt.Errorf("SSH public key format is invalid. Expected ed25519 (ssh-ed25519) or RSA (ssh-rsa) format")
	}

	cfg.SSHKeyPath = strings.TrimSuffix(keyPath, ".pub")

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println()
	fmt.Println("NOTICE: This EC2 instance is provisioned under Viafoura's AWS account and is")
	fmt.Println("intended exclusively for Viafoura engineering work. By continuing, you")
	fmt.Println("acknowledge that personal or unauthorized use of this resource is prohibited")
	fmt.Println("per company policy.")
	fmt.Println()
	fmt.Printf("Configuration saved to %s\n", config.ConfigPath())
	fmt.Println("Run 'clouddesktop up' to provision your cloud desktop.")

	return nil
}

// findAWSProfiles runs 'aws configure list-profiles' and returns the available profile names.
func findAWSProfiles() ([]string, error) {
	out, err := exec.Command("aws", "configure", "list-profiles").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list AWS profiles: %w", err)
	}

	var profiles []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			profiles = append(profiles, line)
		}
	}

	return profiles, nil
}

// findSSHPublicKeys scans ~/.ssh/ for .pub files and returns their absolute paths.
func findSSHPublicKeys() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not determine home directory: %w", err)
	}

	sshDir := filepath.Join(home, ".ssh")
	matches, err := filepath.Glob(filepath.Join(sshDir, "*.pub"))
	if err != nil {
		return nil, fmt.Errorf("failed to scan %s: %w", sshDir, err)
	}

	return matches, nil
}

// shortenPath replaces the home directory prefix with ~ for display.
func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}
