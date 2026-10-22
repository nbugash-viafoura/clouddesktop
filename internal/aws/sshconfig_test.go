package aws

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nbugash-viafoura/clouddesktop/internal/environment"
)

const testProxyScriptPath = "/home/dev/.ssh/clouddesktop-ssm-proxy.sh"

func testSettings() SSHSettings {
	return SSHSettings{
		User:         "ubuntu",
		IdentityFile: "/home/dev/.ssh/id_ed25519",
		AWSProfile:   "test-developers",
		Region:       "us-east-1",
	}
}

func testDeps() SSMProxyDeps {
	return SSMProxyDeps{
		AWSPath:                 "/usr/local/bin/aws",
		SessionManagerPluginDir: "/opt/homebrew/bin",
	}
}

// --- renderProxyScript tests ---

func TestRenderProxyScript(t *testing.T) {
	script := renderProxyScript(testDeps(), testSettings())

	checks := []struct {
		label, want string
	}{
		{"shebang", "#!/bin/bash"},
		{"session-manager-plugin dir in PATH", "/opt/homebrew/bin"},
		{"aws dir in PATH", "/usr/local/bin"},
		{"absolute aws path", "/usr/local/bin/aws ssm start-session"},
		{"target arg", `--target "$1"`},
		{"port arg", `"portNumber=$2"`},
		{"profile", "--profile test-developers"},
		{"region", "--region us-east-1"},
	}
	for _, c := range checks {
		if !strings.Contains(script, c.want) {
			t.Errorf("script missing %s (%q):\n%s", c.label, c.want, script)
		}
	}
}

// --- resolveSSMProxyDepsUsing tests ---

func TestResolveSSMProxyDepsUsing_Success(t *testing.T) {
	lookPath := func(name string) (string, error) {
		switch name {
		case "aws":
			return "/usr/local/bin/aws", nil
		case "session-manager-plugin":
			return "/opt/homebrew/bin/session-manager-plugin", nil
		default:
			return "", fmt.Errorf("not found: %s", name)
		}
	}

	deps, err := resolveSSMProxyDepsUsing(lookPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deps.AWSPath != "/usr/local/bin/aws" {
		t.Errorf("AWSPath = %q, want /usr/local/bin/aws", deps.AWSPath)
	}
	if deps.SessionManagerPluginDir != "/opt/homebrew/bin" {
		t.Errorf("SessionManagerPluginDir = %q, want /opt/homebrew/bin", deps.SessionManagerPluginDir)
	}
}

func TestResolveSSMProxyDepsUsing_MissingAWS(t *testing.T) {
	lookPath := func(name string) (string, error) {
		return "", fmt.Errorf("not found: %s", name)
	}

	_, err := resolveSSMProxyDepsUsing(lookPath)
	if err == nil {
		t.Fatal("expected error when aws is not found")
	}
	if !strings.Contains(err.Error(), "aws CLI not found") {
		t.Errorf("error = %q, want to contain 'aws CLI not found'", err)
	}
}

func TestResolveSSMProxyDepsUsing_MissingSMP(t *testing.T) {
	lookPath := func(name string) (string, error) {
		if name == "aws" {
			return "/usr/local/bin/aws", nil
		}
		return "", fmt.Errorf("not found: %s", name)
	}

	_, err := resolveSSMProxyDepsUsing(lookPath)
	if err == nil {
		t.Fatal("expected error when session-manager-plugin is not found")
	}
	if !strings.Contains(err.Error(), "session-manager-plugin not found") {
		t.Errorf("error = %q, want to contain 'session-manager-plugin not found'", err)
	}
}

// --- writeProxyScriptTo tests ---

func TestWriteProxyScriptTo(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "proxy.sh")

	err := writeProxyScriptTo(scriptPath, testDeps(), testSettings())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("failed to read script: %v", err)
	}
	if !strings.HasPrefix(string(data), "#!/bin/bash") {
		t.Error("script should start with shebang")
	}

	info, _ := os.Stat(scriptPath)
	if info.Mode().Perm() != 0755 {
		t.Errorf("file permissions = %o, want 0755", info.Mode().Perm())
	}
}

func TestWriteProxyScriptTo_WriteError(t *testing.T) {
	err := writeProxyScriptTo("/dev/null/impossible/proxy.sh", testDeps(), testSettings())
	if err == nil {
		t.Fatal("expected error for impossible path")
	}
	if !strings.Contains(err.Error(), "failed to write SSM proxy script") {
		t.Errorf("error = %q, want to contain 'failed to write SSM proxy script'", err)
	}
}

// --- removeProxyScript tests ---

func TestRemoveProxyScript_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "proxy.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/bash"), 0755); err != nil {
		t.Fatal(err)
	}

	err := removeProxyScript(scriptPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(scriptPath); !os.IsNotExist(err) {
		t.Error("script should be removed")
	}
}

func TestRemoveProxyScript_NonExistent(t *testing.T) {
	err := removeProxyScript("/nonexistent/proxy.sh")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
}

// --- replaceOrAppendBlock tests ---

func TestReplaceOrAppendBlock_EmptyContent(t *testing.T) {
	block := "# BEGIN clouddesktop managed block\nHost test\n# END clouddesktop managed block"

	result := replaceOrAppendBlock("", block)

	if !strings.Contains(result, block) {
		t.Errorf("should contain the block:\n%s", result)
	}
	if !strings.HasSuffix(result, "\n") {
		t.Error("result should end with a trailing newline")
	}
}

func TestReplaceOrAppendBlock_AppendToExisting(t *testing.T) {
	existing := "Host bastion\n  HostName 10.0.0.1\n  User ec2-user\n"
	block := sshConfigBeginMarker + "\nHost clouddesktop\n" + sshConfigEndMarker

	result := replaceOrAppendBlock(existing, block)

	if !strings.HasPrefix(result, "Host bastion") {
		t.Errorf("should preserve existing content at the start:\n%s", result)
	}
	if !strings.Contains(result, sshConfigBeginMarker) {
		t.Error("should contain the new managed block")
	}
}

func TestReplaceOrAppendBlock_AppendAddsNewlineIfMissing(t *testing.T) {
	existing := "Host bastion\n  User ec2-user" // no trailing newline
	block := sshConfigBeginMarker + "\nHost cd\n" + sshConfigEndMarker

	result := replaceOrAppendBlock(existing, block)

	idx := strings.Index(result, sshConfigBeginMarker)
	if idx > 0 && result[idx-1] != '\n' {
		t.Error("should insert a newline separator before the appended block")
	}
}

func TestReplaceOrAppendBlock_ReplaceExistingBlock(t *testing.T) {
	oldBlock := sshConfigBeginMarker + "\nHost old\n  HostName i-old\n" + sshConfigEndMarker
	existing := "Host bastion\n  User ec2-user\n\n" + oldBlock + "\n\nHost other\n  User admin\n"
	newBlock := sshConfigBeginMarker + "\nHost new\n  HostName i-new\n" + sshConfigEndMarker

	result := replaceOrAppendBlock(existing, newBlock)

	if strings.Contains(result, "i-old") {
		t.Error("old block content should be replaced")
	}
	if !strings.Contains(result, "i-new") {
		t.Error("new block content should be present")
	}
	if !strings.Contains(result, "Host bastion") {
		t.Error("content before the block should be preserved")
	}
	if !strings.Contains(result, "Host other") {
		t.Error("content after the block should be preserved")
	}
	if strings.Count(result, sshConfigBeginMarker) != 1 {
		t.Errorf("should have exactly one begin marker, got %d", strings.Count(result, sshConfigBeginMarker))
	}
}

func TestReplaceOrAppendBlock_ReplaceDoesNotAccumulateBlankLines(t *testing.T) {
	content := "Host bastion\n  User ec2-user\n"
	block := sshConfigBeginMarker + "\nHost cd\n" + sshConfigEndMarker

	content = replaceOrAppendBlock(content, block)
	content = replaceOrAppendBlock(content, block)
	content = replaceOrAppendBlock(content, block)

	if strings.Contains(content, "\n\n\n") {
		t.Errorf("should not accumulate blank lines after repeated replacements:\n%q", content)
	}
}

// --- sshConfigPath / readSSHConfig tests ---

func TestSSHConfigPath(t *testing.T) {
	sshDir, configPath, err := sshConfigPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(sshDir, ".ssh") {
		t.Errorf("sshDir = %q, want to end with .ssh", sshDir)
	}
	if !strings.HasSuffix(configPath, filepath.Join(".ssh", "config")) {
		t.Errorf("configPath = %q, want to end with .ssh/config", configPath)
	}
}

func TestReadSSHConfig_FileNotExist(t *testing.T) {
	content, err := readSSHConfig("/nonexistent/path/config")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty string for missing file, got: %q", content)
	}
}

func TestReadSSHConfig_ReadError(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := readSSHConfig(tmpDir)
	if err == nil {
		t.Fatal("expected error when reading a directory as a file")
	}
	if !strings.Contains(err.Error(), "failed to read SSH config") {
		t.Errorf("error = %q, want to contain 'failed to read SSH config'", err)
	}
}

func TestReadSSHConfig_Success(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")
	expected := "Host test\n  User admin\n"
	if err := os.WriteFile(configPath, []byte(expected), 0600); err != nil {
		t.Fatal(err)
	}

	content, err := readSSHConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != expected {
		t.Errorf("content = %q, want %q", content, expected)
	}
}

// --- removeManagedBlock tests ---

func TestRemoveManagedBlock_RemovesBlock(t *testing.T) {
	block := sshConfigBeginMarker + "\nHost clouddesktop\n  HostName i-abc\n" + sshConfigEndMarker + "\n"
	content := "Host bastion\n  User ec2-user\n\n" + block + "\nHost other\n  User admin\n"

	result := removeManagedBlock(content)

	if strings.Contains(result, sshConfigBeginMarker) {
		t.Error("managed block should be removed")
	}
	if strings.Contains(result, "i-abc") {
		t.Error("managed block content should be removed")
	}
	if !strings.Contains(result, "Host bastion") {
		t.Error("content before block should be preserved")
	}
	if !strings.Contains(result, "Host other") {
		t.Error("content after block should be preserved")
	}
}

func TestRemoveManagedBlock_NoBlock(t *testing.T) {
	content := "Host bastion\n  User ec2-user\n"
	result := removeManagedBlock(content)
	if result != content {
		t.Errorf("content should be unchanged, got:\n%q", result)
	}
}

func TestRemoveManagedBlock_EmptyContent(t *testing.T) {
	result := removeManagedBlock("")
	if result != "" {
		t.Errorf("expected empty string, got: %q", result)
	}
}

func TestRemoveManagedBlock_OnlyBlock(t *testing.T) {
	content := sshConfigBeginMarker + "\nHost clouddesktop\n" + sshConfigEndMarker + "\n"
	result := removeManagedBlock(content)
	if strings.Contains(result, "clouddesktop") {
		t.Error("block should be removed entirely")
	}
}

// --- backupSSHConfig tests ---

func TestBackupSSHConfig_CreatesBackup(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")
	original := "Host bastion\n  User ec2-user\n"
	if err := os.WriteFile(configPath, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}

	if err := backupSSHConfig(tmpDir, configPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	backupPath := filepath.Join(tmpDir, sshConfigBackupName)
	data, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("backup file not found: %v", err)
	}
	if string(data) != original {
		t.Errorf("backup content = %q, want %q", string(data), original)
	}

	info, _ := os.Stat(backupPath)
	if info.Mode().Perm() != 0600 {
		t.Errorf("backup file mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestBackupSSHConfig_NoSourceFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config") // does not exist

	if err := backupSSHConfig(tmpDir, configPath); err != nil {
		t.Fatalf("expected no error when source does not exist, got: %v", err)
	}

	backupPath := filepath.Join(tmpDir, sshConfigBackupName)
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("backup file should not be created when source does not exist")
	}
}

func TestBackupSSHConfig_EmptySourceFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")
	if err := os.WriteFile(configPath, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}

	if err := backupSSHConfig(tmpDir, configPath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	backupPath := filepath.Join(tmpDir, sshConfigBackupName)
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("backup file should not be created for empty source")
	}
}

// --- multi-environment managed block ---

func testEnvironments() []environment.Environment {
	return []environment.Environment{
		{Name: "main", InstanceID: "i-0abc123"},
		{Name: "build", InstanceID: "i-0def456"},
	}
}

func TestRenderManagedBlock(t *testing.T) {
	block := renderManagedBlock(testEnvironments(), testSettings(), testProxyScriptPath)

	want := sshConfigBeginMarker + `
Host clouddesktop-build
  HostName i-0def456
  User ubuntu
  IdentityFile /home/dev/.ssh/id_ed25519
  ForwardAgent yes
  ProxyCommand /home/dev/.ssh/clouddesktop-ssm-proxy.sh %h %p
  ServerAliveInterval 60
  ServerAliveCountMax 3
  StrictHostKeyChecking accept-new

Host clouddesktop-main clouddesktop
  HostName i-0abc123
  User ubuntu
  IdentityFile /home/dev/.ssh/id_ed25519
  ForwardAgent yes
  ProxyCommand /home/dev/.ssh/clouddesktop-ssm-proxy.sh %h %p
  ServerAliveInterval 60
  ServerAliveCountMax 3
  StrictHostKeyChecking accept-new
` + sshConfigEndMarker

	if block != want {
		t.Errorf("block =\n%s\nwant\n%s", block, want)
	}
}

func TestSyncSSHConfigAt_WritesBlockAndKeepsUserContent(t *testing.T) {
	sshDir := t.TempDir()
	configPath := filepath.Join(sshDir, "config")
	scriptPath := filepath.Join(sshDir, "proxy.sh")
	userContent := "Host github.com\n  User git\n"
	if err := os.WriteFile(configPath, []byte(userContent), 0600); err != nil {
		t.Fatal(err)
	}

	if err := syncSSHConfigAt(sshDir, configPath, scriptPath, testDeps(), testEnvironments(), testSettings()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	content := string(data)
	if !strings.HasPrefix(content, userContent) {
		t.Errorf("content outside the managed block changed:\n%s", content)
	}
	for _, want := range []string{"Host clouddesktop-build", "Host clouddesktop-main clouddesktop"} {
		if !strings.Contains(content, want) {
			t.Errorf("config missing %q:\n%s", want, content)
		}
	}
	if _, err := os.Stat(scriptPath); err != nil {
		t.Errorf("proxy script not written: %v", err)
	}
	backup, err := os.ReadFile(filepath.Join(sshDir, sshConfigBackupName))
	if err != nil {
		t.Fatalf("backup not written: %v", err)
	}
	if string(backup) != userContent {
		t.Errorf("backup = %q, want the content from before the write", backup)
	}
}

func TestSyncSSHConfigAt_ReplacesExistingBlock(t *testing.T) {
	sshDir := t.TempDir()
	configPath := filepath.Join(sshDir, "config")
	scriptPath := filepath.Join(sshDir, "proxy.sh")
	envs := testEnvironments()

	if err := syncSSHConfigAt(sshDir, configPath, scriptPath, testDeps(), envs, testSettings()); err != nil {
		t.Fatal(err)
	}
	if err := syncSSHConfigAt(sshDir, configPath, scriptPath, testDeps(), envs[:1], testSettings()); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(configPath)
	content := string(data)
	if strings.Contains(content, "clouddesktop-build") {
		t.Errorf("stale entry for build was not removed:\n%s", content)
	}
	if strings.Count(content, sshConfigBeginMarker) != 1 {
		t.Errorf("expected exactly one managed block:\n%s", content)
	}
}

func TestSyncSSHConfigAt_NoEnvironmentsRemovesBlockAndScript(t *testing.T) {
	sshDir := t.TempDir()
	configPath := filepath.Join(sshDir, "config")
	scriptPath := filepath.Join(sshDir, "proxy.sh")
	userContent := "Host github.com\n  User git\n"
	if err := os.WriteFile(configPath, []byte(userContent), 0600); err != nil {
		t.Fatal(err)
	}
	if err := syncSSHConfigAt(sshDir, configPath, scriptPath, testDeps(), testEnvironments(), testSettings()); err != nil {
		t.Fatal(err)
	}

	if err := syncSSHConfigAt(sshDir, configPath, scriptPath, SSMProxyDeps{}, nil, testSettings()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	if string(data) != userContent {
		t.Errorf("config = %q, want only the user's content %q", data, userContent)
	}
	if _, err := os.Stat(scriptPath); !os.IsNotExist(err) {
		t.Error("proxy script should be removed when no environments remain")
	}
}

func TestSyncSSHConfigAt_WritesThroughSymlink(t *testing.T) {
	sshDir := t.TempDir()
	dotfiles := t.TempDir()
	realConfig := filepath.Join(dotfiles, "ssh_config")
	configPath := filepath.Join(sshDir, "config")
	if err := os.WriteFile(realConfig, []byte("Host github.com\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realConfig, configPath); err != nil {
		t.Fatal(err)
	}

	if err := syncSSHConfigAt(sshDir, configPath, filepath.Join(sshDir, "proxy.sh"), testDeps(), testEnvironments(), testSettings()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Lstat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("~/.ssh/config should still be a symlink")
	}
	data, _ := os.ReadFile(realConfig)
	if !strings.Contains(string(data), "Host clouddesktop-main clouddesktop") {
		t.Errorf("symlink target was not updated:\n%s", data)
	}
}

func TestHostDefinedOutsideBlockAt(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config")
	content := "Host foo clouddesktop-build\n  User me\nHost clouddesktop-buildx\n" +
		renderManagedBlock([]environment.Environment{{Name: "ci", InstanceID: "i-ci"}}, testSettings(), testProxyScriptPath) + "\n"
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		alias string
		want  bool
	}{
		{alias: "clouddesktop-build", want: true},
		{alias: "clouddesktop-ci", want: false},
		{alias: "clouddesktop-bui", want: false},
		{alias: "clouddesktop-other", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			got, err := hostDefinedOutsideBlockAt(configPath, tt.alias)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("hostDefinedOutsideBlockAt(%q) = %v, want %v", tt.alias, got, tt.want)
			}
		})
	}
}

func TestHostDefinedOutsideBlockAt_MissingConfig(t *testing.T) {
	got, err := hostDefinedOutsideBlockAt(filepath.Join(t.TempDir(), "config"), "clouddesktop-build")
	if err != nil || got {
		t.Errorf("got (%v, %v), want (false, nil) for a missing config", got, err)
	}
}

func TestSyncSSHConfigAt_UnwritableDirectory(t *testing.T) {
	err := syncSSHConfigAt("/dev/null/impossible", "/dev/null/impossible/config", "/dev/null/impossible/proxy.sh", testDeps(), testEnvironments(), testSettings())
	if err == nil {
		t.Fatal("expected an error for an impossible .ssh directory")
	}
}
