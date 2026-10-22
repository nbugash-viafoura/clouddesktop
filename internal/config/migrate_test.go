package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const v1Config = `aws_profile: test-developers
region: us-east-1
instance_type: m7i.xlarge
ssh_public_key: ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 test@host
ssh_key_path: /home/test/.ssh/key
developer_name: john-dev
instance_id: i-0abc123
root_volume_size_gb: 200
`

func writeV1(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(v1Config), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMigrate_V1BecomesV2(t *testing.T) {
	path := writeV1(t)

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom() unexpected error: %v", err)
	}

	if cfg.DefaultEnvironment != "main" {
		t.Errorf("DefaultEnvironment = %q, want main", cfg.DefaultEnvironment)
	}
	if cfg.InstanceID != "" || cfg.RootVolumeSizeGB != 0 {
		t.Errorf("legacy fields not cleared: instance_id=%q root_volume_size_gb=%d", cfg.InstanceID, cfg.RootVolumeSizeGB)
	}
	if cfg.DeveloperName != "john-dev" || cfg.AWSProfile != "test-developers" || cfg.InstanceType != "m7i.xlarge" {
		t.Errorf("shared fields not preserved: %+v", cfg)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if strings.Contains(content, "instance_id") || strings.Contains(content, "root_volume_size_gb") {
		t.Errorf("migrated file still has legacy fields:\n%s", content)
	}
	if !strings.Contains(content, "default_environment: main") {
		t.Errorf("migrated file lacks default_environment:\n%s", content)
	}
}

func TestMigrate_WritesByteIdenticalBackup(t *testing.T) {
	path := writeV1(t)

	if _, err := LoadFrom(path); err != nil {
		t.Fatalf("LoadFrom() unexpected error: %v", err)
	}

	backupPath := path + ".v1-backup"
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("backup not written: %v", err)
	}
	if string(backup) != v1Config {
		t.Errorf("backup differs from the original:\n%s", backup)
	}
	info, _ := os.Stat(backupPath)
	if info.Mode().Perm() != 0600 {
		t.Errorf("backup mode = %o, want 0600", info.Mode().Perm())
	}
}

func TestMigrate_KeepsExistingBackup(t *testing.T) {
	path := writeV1(t)
	backupPath := path + ".v1-backup"
	if err := os.WriteFile(backupPath, []byte("older backup"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadFrom(path); err != nil {
		t.Fatalf("LoadFrom() unexpected error: %v", err)
	}

	backup, _ := os.ReadFile(backupPath)
	if string(backup) != "older backup" {
		t.Errorf("existing backup was overwritten: %q", backup)
	}
}

func TestMigrate_SecondLoadIsNoOp(t *testing.T) {
	path := writeV1(t)
	if _, err := LoadFrom(path); err != nil {
		t.Fatal(err)
	}
	backupPath := path + ".v1-backup"
	if err := os.Remove(backupPath); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadFrom(path); err != nil {
		t.Fatalf("second LoadFrom() unexpected error: %v", err)
	}

	info, _ := os.Stat(path)
	if !info.ModTime().Equal(past) {
		t.Error("config was rewritten on the second load")
	}
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("a new backup was written on the second load")
	}
}

func TestMigrate_FailureLeavesV1Intact(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("directory permissions are not enforced for root")
	}
	path := writeV1(t)
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0700) })

	if _, err := LoadFrom(path); err == nil {
		t.Fatal("expected an error when the config directory is read-only")
	}

	data, _ := os.ReadFile(path)
	if string(data) != v1Config {
		t.Errorf("v1 config changed after a failed migration:\n%s", data)
	}
}

func TestMigrate_V2FileIsNeverRewritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := validConfig()
	cfg.DefaultEnvironment = "build"
	if err := SaveTo(path, cfg); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadFrom(path); err != nil {
		t.Fatalf("LoadFrom() unexpected error: %v", err)
	}

	info, _ := os.Stat(path)
	if !info.ModTime().Equal(past) {
		t.Error("v2 config was rewritten")
	}
	if _, err := os.Stat(path + ".v1-backup"); !os.IsNotExist(err) {
		t.Error("a backup was written for a v2 config")
	}
}
