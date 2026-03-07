package aws

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testProxyScriptPath = "/home/dev/.ssh/clouddesktop-ssm-proxy.sh"

func testEntry() SSHConfigEntry {
	return SSHConfigEntry{
		HostAlias:    "clouddesktop",
		InstanceID:   "i-0abc123def456",
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

// --- renderSSHConfigBlock tests ---

func TestRenderSSHConfigBlock(t *testing.T) {
	entry := testEntry()
	block := renderSSHConfigBlock(entry, testProxyScriptPath)

	if !strings.HasPrefix(block, sshConfigBeginMarker) {
		t.Errorf("block should start with begin marker, got:\n%s", block)
	}
	if !strings.HasSuffix(block, sshConfigEndMarker) {
		t.Errorf("block should end with end marker, got:\n%s", block)
	}

	checks := []struct {
		label, want string
	}{
		{"Host alias", "Host clouddesktop"},
		{"HostName", "HostName i-0abc123def456"},
		{"User", "User ubuntu"},
		{"IdentityFile", "IdentityFile /home/dev/.ssh/id_ed25519"},
		{"ForwardAgent", "ForwardAgent yes"},
		{"ProxyCommand script path", "ProxyCommand " + testProxyScriptPath},
		{"ProxyCommand %h", "%h"},
		{"ProxyCommand %p", "%p"},
		{"ServerAliveInterval", "ServerAliveInterval 60"},
		{"StrictHostKeyChecking", "StrictHostKeyChecking accept-new"},
	}
	for _, c := range checks {
		if !strings.Contains(block, c.want) {
			t.Errorf("block missing %s (%q):\n%s", c.label, c.want, block)
		}
	}
}

func TestRenderSSHConfigBlock_NoInlineAWSCommand(t *testing.T) {
	block := renderSSHConfigBlock(testEntry(), testProxyScriptPath)
	if strings.Contains(block, "aws ssm start-session") {
		t.Error("ProxyCommand should reference the proxy script, not inline aws ssm start-session")
	}
}

// --- renderProxyScript tests ---

func TestRenderProxyScript(t *testing.T) {
	deps := testDeps()
	entry := testEntry()
	script := renderProxyScript(deps, entry)

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

	err := writeProxyScriptTo(scriptPath, testDeps(), testEntry())
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
	err := writeProxyScriptTo("/dev/null/impossible/proxy.sh", testDeps(), testEntry())
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

// --- Filesystem tests for writeSSHConfigWithProxy ---

func TestWriteSSHConfigWithProxy_CreatesNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	configPath := filepath.Join(sshDir, "config")
	scriptPath := filepath.Join(sshDir, proxyScriptName)

	err := writeSSHConfigWithProxy(sshDir, configPath, scriptPath, testDeps(), testEntry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify SSH config
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Host clouddesktop") {
		t.Error("config should contain Host clouddesktop")
	}
	if !strings.Contains(content, sshConfigBeginMarker) {
		t.Error("config should contain begin marker")
	}
	if !strings.Contains(content, scriptPath) {
		t.Errorf("config should reference proxy script path %q", scriptPath)
	}
	if strings.Contains(content, "aws ssm start-session") {
		t.Error("config should not contain inline aws ssm command")
	}

	info, _ := os.Stat(configPath)
	if info.Mode().Perm() != 0600 {
		t.Errorf("config permissions = %o, want 0600", info.Mode().Perm())
	}

	// Verify proxy script
	scriptData, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("failed to read proxy script: %v", err)
	}
	if !strings.HasPrefix(string(scriptData), "#!/bin/bash") {
		t.Error("proxy script should start with shebang")
	}
	scriptInfo, _ := os.Stat(scriptPath)
	if scriptInfo.Mode().Perm() != 0755 {
		t.Errorf("script permissions = %o, want 0755", scriptInfo.Mode().Perm())
	}
}

func TestWriteSSHConfigWithProxy_CreatesSSHDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, "nested", ".ssh")
	configPath := filepath.Join(sshDir, "config")
	scriptPath := filepath.Join(sshDir, proxyScriptName)

	err := writeSSHConfigWithProxy(sshDir, configPath, scriptPath, testDeps(), testEntry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dirInfo, err := os.Stat(sshDir)
	if err != nil {
		t.Fatalf("ssh dir not created: %v", err)
	}
	if dirInfo.Mode().Perm() != 0700 {
		t.Errorf("directory permissions = %o, want 0700", dirInfo.Mode().Perm())
	}
}

func TestWriteSSHConfigWithProxy_UpdatesExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	configPath := filepath.Join(sshDir, "config")
	scriptPath := filepath.Join(sshDir, proxyScriptName)

	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatal(err)
	}
	existing := "Host bastion\n  HostName 10.0.0.1\n  User ec2-user\n"
	if err := os.WriteFile(configPath, []byte(existing), 0600); err != nil {
		t.Fatal(err)
	}

	err := writeSSHConfigWithProxy(sshDir, configPath, scriptPath, testDeps(), testEntry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	content := string(data)
	if !strings.Contains(content, "Host bastion") {
		t.Error("existing content should be preserved")
	}
	if !strings.Contains(content, "Host clouddesktop") {
		t.Error("new block should be appended")
	}
}

func TestWriteSSHConfigWithProxy_ReplacesExistingBlock(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	configPath := filepath.Join(sshDir, "config")
	scriptPath := filepath.Join(sshDir, proxyScriptName)

	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatal(err)
	}
	oldEntry := testEntry()
	oldEntry.InstanceID = "i-old-instance"
	oldBlock := renderSSHConfigBlock(oldEntry, scriptPath)
	existing := "Host bastion\n  User ec2-user\n\n" + oldBlock + "\n"
	if err := os.WriteFile(configPath, []byte(existing), 0600); err != nil {
		t.Fatal(err)
	}

	err := writeSSHConfigWithProxy(sshDir, configPath, scriptPath, testDeps(), testEntry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	content := string(data)
	if strings.Contains(content, "i-old-instance") {
		t.Error("old instance ID should be replaced")
	}
	if !strings.Contains(content, "i-0abc123def456") {
		t.Error("new instance ID should be present")
	}
	if strings.Count(content, sshConfigBeginMarker) != 1 {
		t.Error("should have exactly one managed block")
	}
}

func TestWriteSSHConfigWithProxy_MkdirError(t *testing.T) {
	err := writeSSHConfigWithProxy("/dev/null/impossible", "/dev/null/impossible/config", "/dev/null/impossible/proxy.sh", testDeps(), testEntry())
	if err == nil {
		t.Fatal("expected error for impossible directory")
	}
	if !strings.Contains(err.Error(), "failed to create .ssh directory") {
		t.Errorf("error = %q, want to contain 'failed to create .ssh directory'", err)
	}
}

func TestWriteSSHConfigWithProxy_WriteFileError(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	configPath := filepath.Join(sshDir, "config")
	scriptPath := filepath.Join(sshDir, proxyScriptName)

	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(configPath, 0400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(configPath, 0600) })

	err := writeSSHConfigWithProxy(sshDir, configPath, scriptPath, testDeps(), testEntry())
	if err == nil {
		t.Fatal("expected error writing to read-only file")
	}
	if !strings.Contains(err.Error(), "failed to write SSH config") {
		t.Errorf("error = %q, want to contain 'failed to write SSH config'", err)
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

// --- removeManagedBlock and removeSSHConfigFrom tests ---

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

func TestRemoveSSHConfigFrom_RemovesBlock(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	configPath := filepath.Join(sshDir, "config")

	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatal(err)
	}

	block := renderSSHConfigBlock(testEntry(), "/tmp/proxy.sh")
	content := "Host bastion\n  User ec2-user\n\n" + block + "\n"
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	err := removeSSHConfigFrom(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	result := string(data)
	if strings.Contains(result, sshConfigBeginMarker) {
		t.Error("managed block should be removed from file")
	}
	if !strings.Contains(result, "Host bastion") {
		t.Error("other SSH config should be preserved")
	}
}

func TestRemoveSSHConfigFrom_NoFile(t *testing.T) {
	err := removeSSHConfigFrom("/nonexistent/path/config")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
}

func TestRemoveSSHConfigFrom_NoBlock(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")
	content := "Host bastion\n  User ec2-user\n"
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	err := removeSSHConfigFrom(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(configPath)
	if string(data) != content {
		t.Error("file should be unchanged when no managed block exists")
	}
}
