package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	vfaws "github.com/nbugash-viafoura/clouddesktop/internal/aws"
	"github.com/nbugash-viafoura/clouddesktop/internal/config"
	"github.com/nbugash-viafoura/clouddesktop/internal/environment"
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
	if existing != nil {
		envs, err := listEnvironments(existing.AWSProfile, existing.Region, existing.DeveloperName)
		if err != nil {
			return fmt.Errorf("failed to check for existing environments: %w", err)
		}
		if len(envs) > 0 {
			return fmt.Errorf("clouddesktop is already initialized and you have environments: %s\nDestroy them first if you need to change your developer name or SSH key",
				strings.Join(environment.Names(envs), ", "))
		}
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

	// Discover environments this developer already owns (for example on a new
	// laptop) so access to them can be restored without re-creating anything.
	envs, err := listEnvironments(cfg.AWSProfile, cfg.Region, cfg.DeveloperName)
	if err != nil {
		fmt.Printf("Warning: could not look up existing environments: %s\n", err)
		envs = nil
	}
	if len(envs) > 0 {
		fmt.Printf("Found %d environment(s): %s\n", len(envs), strings.Join(environment.Names(envs), ", "))
		fmt.Println("Select the SSH key these environments were created with.")
	}

	// Default instance type for new environments (select), defaulting to m7i.xlarge.
	instanceType := "m7i.xlarge"
	instanceTypeOptions := make([]huh.Option[string], len(config.ValidInstanceTypes))
	for i, it := range config.ValidInstanceTypes {
		instanceTypeOptions[i] = huh.NewOption(it.Label, it.Value)
	}

	selectForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Default instance type for new environments").
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

	defaultEnv, err := chooseDefaultEnvironment(envs)
	if err != nil {
		return err
	}
	cfg.DefaultEnvironment = defaultEnv

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if len(envs) > 0 {
		if err := vfaws.SyncSSHConfig(envs, sshSettings(cfg)); err != nil {
			fmt.Printf("Warning: failed to update SSH config: %s\n", err)
			fmt.Println("  Run 'clouddesktop up <name>' to retry.")
		}
	}

	fmt.Println()
	fmt.Println("NOTICE: This EC2 instance is provisioned under Viafoura's AWS account and is")
	fmt.Println("intended exclusively for Viafoura engineering work. By continuing, you")
	fmt.Println("acknowledge that personal or unauthorized use of this resource is prohibited")
	fmt.Println("per company policy.")
	fmt.Println()
	fmt.Printf("Configuration saved to %s\n", config.ConfigPath())
	if len(envs) > 0 {
		fmt.Println("Run 'clouddesktop list' to see your environments.")
	} else {
		fmt.Println("Run 'clouddesktop up' to provision your cloud desktop.")
	}

	return nil
}

// chooseDefaultEnvironment picks the default after discovery: the only
// environment when there is one, the developer's choice when there are several.
func chooseDefaultEnvironment(envs []environment.Environment) (string, error) {
	switch len(envs) {
	case 0:
		return "", nil
	case 1:
		return envs[0].Name, nil
	}

	names := environment.Names(envs)
	options := make([]huh.Option[string], len(names))
	for i, name := range names {
		options[i] = huh.NewOption(name, name)
	}

	choice := names[0]
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Default environment").
				Options(options...).
				Value(&choice),
		),
	)
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("init cancelled: %w", err)
	}
	return choice, nil
}

// listEnvironments returns the developer's environments using the given AWS profile.
func listEnvironments(profile, region, developerName string) ([]environment.Environment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := vfaws.ValidateSession(ctx, profile, region); err != nil {
		return nil, err
	}
	ec2Client, err := vfaws.NewEC2Client(ctx, profile, region)
	if err != nil {
		return nil, err
	}
	return ec2Client.ListEnvironments(ctx, developerName)
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
