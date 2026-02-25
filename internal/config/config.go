package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the clouddesktop configuration stored in ~/.clouddesktop/config.yaml.
type Config struct {
	AWSProfile      string `yaml:"aws_profile"`
	Region          string `yaml:"region"`
	InstanceType    string `yaml:"instance_type"`
	SSHPublicKey    string `yaml:"ssh_public_key"`
	SSHKeyPath      string `yaml:"ssh_key_path"`
	ShellConfigPath string `yaml:"shell_config_path,omitempty"`
	DeveloperName   string `yaml:"developer_name"`
	InstanceID      string `yaml:"instance_id,omitempty"`
}

var (
	// ErrConfigNotFound is returned when the config file does not exist.
	ErrConfigNotFound = errors.New("config file not found - run 'clouddesktop init' to initialize")
)

// Valid AWS regions (common subset; add more as needed)
var validAWSRegions = map[string]bool{
	"us-east-1":      true,
	"us-east-2":      true,
	"us-west-1":      true,
	"us-west-2":      true,
	"eu-central-1":   true,
	"eu-west-1":      true,
	"ap-northeast-1": true,
	"ap-southeast-1": true,
}

// Valid EC2 instance types (m7i family for clouddesktop)
var validInstanceTypes = map[string]bool{
	"m7i.large":    true,
	"m7i.xlarge":   true,
	"m7i.2xlarge":  true,
	"m7i.4xlarge":  true,
	"m6i.large":    true,
	"m6i.xlarge":   true,
	"m6i.2xlarge":  true,
	"t3.large":     true,
	"t3.xlarge":    true,
	"t3.2xlarge":   true,
	"t4g.large":    true,
	"t4g.xlarge":   true,
	"t4g.2xlarge":  true,
}

// ConfigPath returns the absolute path to the clouddesktop config file.
func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".clouddesktop", "config.yaml")
}

// Validate checks that the config has all required fields and valid values.
func (c *Config) Validate() error {
	if c.AWSProfile == "" {
		return errors.New("aws_profile is required (default: test-developers)")
	}

	if c.Region == "" {
		return errors.New("region is required (default: us-east-1)")
	}
	if !validAWSRegions[c.Region] {
		return fmt.Errorf("region '%s' is not a valid AWS region", c.Region)
	}

	if c.InstanceType == "" {
		return errors.New("instance_type is required (default: m7i.xlarge)")
	}
	if !validInstanceTypes[c.InstanceType] {
		return fmt.Errorf("instance_type '%s' is not valid (supported: m7i.xlarge, m7i.2xlarge, etc.)", c.InstanceType)
	}

	if c.DeveloperName == "" {
		return errors.New("developer_name is required")
	}
	// Validate lowercase and alphanumeric
	if !isValidDeveloperName(c.DeveloperName) {
		return errors.New("developer_name must be lowercase alphanumeric with hyphens only (e.g. 'john-dev')")
	}

	if c.SSHPublicKey == "" {
		return errors.New("ssh_public_key is required")
	}
	// Validate SSH key format (should start with ssh-ed25519 or ssh-rsa)
	if !strings.HasPrefix(c.SSHPublicKey, "ssh-") {
		return errors.New("ssh_public_key appears invalid (should start with 'ssh-rsa' or 'ssh-ed25519')")
	}

	if c.SSHKeyPath == "" {
		return errors.New("ssh_key_path is required")
	}

	return nil
}

// isValidDeveloperName checks if a developer name follows naming conventions.
func isValidDeveloperName(name string) bool {
	// Must be lowercase, alphanumeric, and hyphens only
	matched, _ := regexp.MatchString(`^[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`, name)
	return matched
}

// Load reads and parses the clouddesktop configuration from disk.
func Load() (*Config, error) {
	path := ConfigPath()
	if path == "" {
		return nil, errors.New("could not determine home directory")
	}
	return LoadFrom(path)
}

// LoadFrom reads and parses the clouddesktop configuration from the given file path.
func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrConfigNotFound
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config file is invalid YAML: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// Save writes the clouddesktop configuration to the default config path.
// The config file is saved with mode 0600 since it contains sensitive data (SSH public key).
func Save(cfg *Config) error {
	path := ConfigPath()
	if path == "" {
		return errors.New("could not determine home directory")
	}
	return SaveTo(path, cfg)
}

// SaveTo writes the clouddesktop configuration to the specified path, creating
// the parent directory if needed. The file is written with mode 0600.
func SaveTo(path string, cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("cannot save invalid config: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
