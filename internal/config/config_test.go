package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateRequiredFields verifies that validation catches missing required fields.
func TestValidateRequiredFields(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *Config
		wantError bool
		errorMsg  string
	}{
		{
			name: "all fields present",
			cfg: &Config{
				AWSProfile:    "test-developers",
				Region:        "us-east-1",
				InstanceType:  "m7i.xlarge",
				SSHPublicKey:  "ssh-ed25519 AAAAC test",
				SSHKeyPath:    "/home/test/.ssh/key",
				DeveloperName: "john-dev",
				StateS3Bucket: "bucket",
			},
			wantError: false,
		},
		{
			name: "missing aws_profile",
			cfg: &Config{
				Region:        "us-east-1",
				InstanceType:  "m7i.xlarge",
				SSHPublicKey:  "ssh-ed25519 AAAAC test",
				SSHKeyPath:    "/home/test/.ssh/key",
				DeveloperName: "john-dev",
				StateS3Bucket: "bucket",
			},
			wantError: true,
			errorMsg:  "aws_profile is required",
		},
		{
			name: "missing region",
			cfg: &Config{
				AWSProfile:    "test-developers",
				InstanceType:  "m7i.xlarge",
				SSHPublicKey:  "ssh-ed25519 AAAAC test",
				SSHKeyPath:    "/home/test/.ssh/key",
				DeveloperName: "john-dev",
				StateS3Bucket: "bucket",
			},
			wantError: true,
			errorMsg:  "region is required",
		},
		{
			name: "missing developer_name",
			cfg: &Config{
				AWSProfile:    "test-developers",
				Region:        "us-east-1",
				InstanceType:  "m7i.xlarge",
				SSHPublicKey:  "ssh-ed25519 AAAAC test",
				SSHKeyPath:    "/home/test/.ssh/key",
				StateS3Bucket: "bucket",
			},
			wantError: true,
			errorMsg:  "developer_name is required",
		},
		{
			name: "missing ssh_key_path",
			cfg: &Config{
				AWSProfile:    "test-developers",
				Region:        "us-east-1",
				InstanceType:  "m7i.xlarge",
				SSHPublicKey:  "ssh-ed25519 AAAAC test",
				SSHKeyPath:    "",
				DeveloperName: "john-dev",
				StateS3Bucket: "bucket",
			},
			wantError: true,
			errorMsg:  "ssh_key_path is required",
		},
		{
			name: "missing state_s3_bucket",
			cfg: &Config{
				AWSProfile:    "test-developers",
				Region:        "us-east-1",
				InstanceType:  "m7i.xlarge",
				SSHPublicKey:  "ssh-ed25519 AAAAC test",
				SSHKeyPath:    "/home/test/.ssh/key",
				DeveloperName: "john-dev",
				StateS3Bucket: "",
			},
			wantError: true,
			errorMsg:  "state_s3_bucket is required",
		},
		{
			name: "missing instance_type",
			cfg: &Config{
				AWSProfile:    "test-developers",
				Region:        "us-east-1",
				InstanceType:  "",
				SSHPublicKey:  "ssh-ed25519 AAAAC test",
				SSHKeyPath:    "/home/test/.ssh/key",
				DeveloperName: "john-dev",
				StateS3Bucket: "bucket",
			},
			wantError: true,
			errorMsg:  "instance_type is required",
		},
		{
			name: "missing ssh_public_key",
			cfg: &Config{
				AWSProfile:    "test-developers",
				Region:        "us-east-1",
				InstanceType:  "m7i.xlarge",
				SSHPublicKey:  "",
				SSHKeyPath:    "/home/test/.ssh/key",
				DeveloperName: "john-dev",
				StateS3Bucket: "bucket",
			},
			wantError: true,
			errorMsg:  "ssh_public_key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
			if tt.wantError && err != nil && !strings.Contains(err.Error(), tt.errorMsg) {
				t.Errorf("Validate() error = %q, want to contain %q", err.Error(), tt.errorMsg)
			}
		})
	}
}

// TestValidateAWSRegion verifies that invalid regions are rejected.
func TestValidateAWSRegion(t *testing.T) {
	tests := []struct {
		name      string
		region    string
		wantError bool
	}{
		{"valid us-east-1", "us-east-1", false},
		{"valid us-west-2", "us-west-2", false},
		{"valid eu-west-1", "eu-west-1", false},
		{"invalid region", "invalid-region", true},
		{"empty region", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				AWSProfile:    "test-developers",
				Region:        tt.region,
				InstanceType:  "m7i.xlarge",
				SSHPublicKey:  "ssh-ed25519 AAAAC test",
				SSHKeyPath:    "/home/test/.ssh/key",
				DeveloperName: "john-dev",
				StateS3Bucket: "bucket",
			}
			err := cfg.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
			if tt.wantError && err != nil && !strings.Contains(err.Error(), "region") {
				t.Errorf("Validate() error = %q, want to contain 'region'", err.Error())
			}
		})
	}
}

// TestValidateInstanceType verifies that invalid instance types are rejected.
func TestValidateInstanceType(t *testing.T) {
	tests := []struct {
		name         string
		instanceType string
		wantError    bool
	}{
		{"valid m7i.xlarge", "m7i.xlarge", false},
		{"valid m7i.2xlarge", "m7i.2xlarge", false},
		{"valid t3.large", "t3.large", false},
		{"invalid instance type", "invalid.large", true},
		{"empty instance type", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				AWSProfile:    "test-developers",
				Region:        "us-east-1",
				InstanceType:  tt.instanceType,
				SSHPublicKey:  "ssh-ed25519 AAAAC test",
				SSHKeyPath:    "/home/test/.ssh/key",
				DeveloperName: "john-dev",
				StateS3Bucket: "bucket",
			}
			err := cfg.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
			if tt.wantError && err != nil && !strings.Contains(err.Error(), "instance_type") {
				t.Errorf("Validate() error = %q, want to contain 'instance_type'", err.Error())
			}
		})
	}
}

// TestValidateDeveloperName verifies that developer name format is validated.
func TestValidateDeveloperName(t *testing.T) {
	tests := []struct {
		name      string
		devName   string
		wantError bool
	}{
		{"valid lowercase", "john-dev", false},
		{"valid with numbers", "dev-123", false},
		{"single char", "a", false},
		{"invalid uppercase", "John-Dev", true},
		{"invalid spaces", "john dev", true},
		{"invalid symbols", "john@dev", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				AWSProfile:    "test-developers",
				Region:        "us-east-1",
				InstanceType:  "m7i.xlarge",
				SSHPublicKey:  "ssh-ed25519 AAAAC test",
				SSHKeyPath:    "/home/test/.ssh/key",
				DeveloperName: tt.devName,
				StateS3Bucket: "bucket",
			}
			err := cfg.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
			if tt.wantError && err != nil && !strings.Contains(err.Error(), "developer_name") {
				t.Errorf("Validate() error = %q, want to contain 'developer_name'", err.Error())
			}
		})
	}
}

// TestValidateSSHPublicKey verifies that SSH public key format is validated.
func TestValidateSSHPublicKey(t *testing.T) {
	tests := []struct {
		name      string
		sshKey    string
		wantError bool
	}{
		{"valid ed25519", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 test@host", false},
		{"valid rsa", "ssh-rsa AAAAB3NzaC1yc2E test@host", false},
		{"invalid format", "invalid-key-format", true},
		{"no ssh- prefix", "ed25519 AAAAC3NzaC1lZDI1NTE5 test@host", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				AWSProfile:    "test-developers",
				Region:        "us-east-1",
				InstanceType:  "m7i.xlarge",
				SSHPublicKey:  tt.sshKey,
				SSHKeyPath:    "/home/test/.ssh/key",
				DeveloperName: "john-dev",
				StateS3Bucket: "bucket",
			}
			err := cfg.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
			if tt.wantError && err != nil && !strings.Contains(err.Error(), "ssh_public_key") {
				t.Errorf("Validate() error = %q, want to contain 'ssh_public_key'", err.Error())
			}
		})
	}
}

// TestSaveToWithFilePermissions verifies that SaveTo creates the file with 0600 permissions
// and that the content round-trips correctly.
func TestSaveToWithFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "subdir", "config.yaml")

	cfg := &Config{
		AWSProfile:    "test-developers",
		Region:        "us-east-1",
		InstanceType:  "m7i.xlarge",
		SSHPublicKey:  "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 test@host",
		SSHKeyPath:    "/home/test/.ssh/key",
		DeveloperName: "john-dev",
		StateS3Bucket: "bucket",
	}

	err := SaveTo(configFile, cfg)
	if err != nil {
		t.Fatalf("SaveTo() unexpected error: %v", err)
	}

	// Verify file exists
	info, err := os.Stat(configFile)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	// Verify permissions are 0600
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}

	// Verify parent directory was created with 0700
	dirInfo, err := os.Stat(filepath.Dir(configFile))
	if err != nil {
		t.Fatalf("config directory not created: %v", err)
	}
	dirPerm := dirInfo.Mode().Perm()
	if dirPerm != 0700 {
		t.Errorf("directory permissions = %o, want 0700", dirPerm)
	}

	// Verify content round-trips: read back the file and check fields
	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "test-developers") {
		t.Error("saved config should contain aws_profile value")
	}
	if !strings.Contains(content, "john-dev") {
		t.Error("saved config should contain developer_name value")
	}
}

// TestSaveToRejectsInvalidConfig verifies that SaveTo refuses to write invalid configs.
func TestSaveToRejectsInvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	cfg := &Config{
		AWSProfile: "test-developers",
		// Missing required fields
	}

	err := SaveTo(configFile, cfg)
	if err == nil {
		t.Fatal("SaveTo() should reject invalid config")
	}
	if !strings.Contains(err.Error(), "cannot save invalid config") {
		t.Errorf("error should mention invalid config, got: %v", err)
	}

	// Verify file was NOT created
	if _, err := os.Stat(configFile); !os.IsNotExist(err) {
		t.Error("config file should not be created for invalid config")
	}
}

// TestLoadFrom_ValidConfig verifies loading a valid config file.
func TestLoadFrom_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `aws_profile: test-developers
region: us-east-1
instance_type: m7i.xlarge
ssh_public_key: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 test@host"
ssh_key_path: /home/test/.ssh/key
developer_name: john-dev
state_s3_bucket: bucket
`
	if err := os.WriteFile(configFile, []byte(yamlContent), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(configFile)
	if err != nil {
		t.Fatalf("LoadFrom() unexpected error: %v", err)
	}
	if cfg.AWSProfile != "test-developers" {
		t.Errorf("AWSProfile = %q, want test-developers", cfg.AWSProfile)
	}
	if cfg.DeveloperName != "john-dev" {
		t.Errorf("DeveloperName = %q, want john-dev", cfg.DeveloperName)
	}
}

// TestLoadFrom_FileNotFound verifies that missing file returns ErrConfigNotFound.
func TestLoadFrom_FileNotFound(t *testing.T) {
	_, err := LoadFrom("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if err != ErrConfigNotFound {
		t.Errorf("error = %v, want ErrConfigNotFound", err)
	}
}

// TestLoadFrom_InvalidYAML verifies that malformed YAML returns a parse error.
func TestLoadFrom_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(configFile, []byte("{{invalid yaml:::"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFrom(configFile)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "invalid YAML") {
		t.Errorf("error = %q, want to contain 'invalid YAML'", err)
	}
}

// TestLoadFrom_ValidationFailure verifies that valid YAML with missing fields fails validation.
func TestLoadFrom_ValidationFailure(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `aws_profile: test-developers
region: us-east-1
`
	if err := os.WriteFile(configFile, []byte(yamlContent), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFrom(configFile)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "config validation failed") {
		t.Errorf("error = %q, want to contain 'config validation failed'", err)
	}
}

// TestLoadFrom_ReadError verifies that a non-NotExist read error is reported.
func TestLoadFrom_ReadError(t *testing.T) {
	// Using a directory path triggers a read error that is not os.IsNotExist.
	tmpDir := t.TempDir()
	_, err := LoadFrom(tmpDir)
	if err == nil {
		t.Fatal("expected error when reading a directory")
	}
	if strings.Contains(err.Error(), "config file not found") {
		t.Error("should not be ErrConfigNotFound for a directory read error")
	}
	if !strings.Contains(err.Error(), "failed to read config file") {
		t.Errorf("error = %q, want to contain 'failed to read config file'", err)
	}
}

// TestSaveToWriteError verifies SaveTo reports errors when the file can't be written.
func TestSaveToWriteError(t *testing.T) {
	// Create a read-only directory to prevent file creation inside it.
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0500); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		AWSProfile:    "test-developers",
		Region:        "us-east-1",
		InstanceType:  "m7i.xlarge",
		SSHPublicKey:  "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 test@host",
		SSHKeyPath:    "/home/test/.ssh/key",
		DeveloperName: "john-dev",
		StateS3Bucket: "bucket",
	}

	err := SaveTo(filepath.Join(readOnlyDir, "subdir", "config.yaml"), cfg)
	if err == nil {
		t.Fatal("expected error writing to read-only directory")
	}
}

// TestConfigPath verifies that the config path is constructed correctly.
func TestConfigPath(t *testing.T) {
	path := ConfigPath()
	if path == "" {
		t.Error("ConfigPath() returned empty string")
	}
	if !strings.Contains(path, ".clouddesktop/config.yaml") {
		t.Errorf("ConfigPath() = %q, want to contain '.clouddesktop/config.yaml'", path)
	}
}

// TestErrConfigNotFound verifies that the error is defined.
func TestErrConfigNotFound(t *testing.T) {
	if ErrConfigNotFound == nil {
		t.Error("ErrConfigNotFound is nil")
	}
	if !strings.Contains(ErrConfigNotFound.Error(), "config file not found") {
		t.Errorf("ErrConfigNotFound message = %q, want to contain 'config file not found'", ErrConfigNotFound.Error())
	}
}

