package aws

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderSSHConfigBlock(t *testing.T) {
	entry := SSHConfigEntry{
		HostAlias:    "clouddesktop",
		InstanceID:   "i-0abc123def456",
		User:         "ubuntu",
		IdentityFile: "/home/dev/.ssh/id_ed25519",
		AWSProfile:   "test-developers",
		Region:       "us-east-1",
	}

	block := renderSSHConfigBlock(entry)

	// Must be wrapped in markers
	if !strings.HasPrefix(block, sshConfigBeginMarker) {
		t.Errorf("block should start with begin marker, got:\n%s", block)
	}
	if !strings.HasSuffix(block, sshConfigEndMarker) {
		t.Errorf("block should end with end marker, got:\n%s", block)
	}

	// Verify key fields appear in the output
	checks := []struct {
		label, want string
	}{
		{"Host alias", "Host clouddesktop"},
		{"HostName", "HostName i-0abc123def456"},
		{"User", "User ubuntu"},
		{"IdentityFile", "IdentityFile /home/dev/.ssh/id_ed25519"},
		{"ForwardAgent", "ForwardAgent yes"},
		{"ProxyCommand profile", "--profile test-developers"},
		{"ProxyCommand region", "--region us-east-1"},
		{"ServerAliveInterval", "ServerAliveInterval 60"},
		{"StrictHostKeyChecking", "StrictHostKeyChecking accept-new"},
	}
	for _, c := range checks {
		if !strings.Contains(block, c.want) {
			t.Errorf("block missing %s (%q):\n%s", c.label, c.want, block)
		}
	}

	// ProxyCommand should use literal %h and %p (SSH placeholders), not Go fmt verbs
	if !strings.Contains(block, "--target %h") {
		t.Error("ProxyCommand should contain literal %h for SSH host substitution")
	}
	if !strings.Contains(block, "portNumber=%p") {
		t.Errorf("ProxyCommand should contain literal SSH port placeholder, got:\n%s", block)
	}
}

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

	// Original content preserved
	if !strings.HasPrefix(result, "Host bastion") {
		t.Errorf("should preserve existing content at the start:\n%s", result)
	}
	// New block appended
	if !strings.Contains(result, sshConfigBeginMarker) {
		t.Error("should contain the new managed block")
	}
}

func TestReplaceOrAppendBlock_AppendAddsNewlineIfMissing(t *testing.T) {
	existing := "Host bastion\n  User ec2-user" // no trailing newline
	block := sshConfigBeginMarker + "\nHost cd\n" + sshConfigEndMarker

	result := replaceOrAppendBlock(existing, block)

	// Should not smash the block onto the last line of existing content
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

	// Old content gone
	if strings.Contains(result, "i-old") {
		t.Error("old block content should be replaced")
	}
	// New content present
	if !strings.Contains(result, "i-new") {
		t.Error("new block content should be present")
	}
	// Surrounding content preserved
	if !strings.Contains(result, "Host bastion") {
		t.Error("content before the block should be preserved")
	}
	if !strings.Contains(result, "Host other") {
		t.Error("content after the block should be preserved")
	}
	// Only one begin marker (no duplication)
	if strings.Count(result, sshConfigBeginMarker) != 1 {
		t.Errorf("should have exactly one begin marker, got %d", strings.Count(result, sshConfigBeginMarker))
	}
}

func TestReplaceOrAppendBlock_ReplaceDoesNotAccumulateBlankLines(t *testing.T) {
	// Simulate replacing multiple times -- each replace should not add extra blank lines
	content := "Host bastion\n  User ec2-user\n"
	block := sshConfigBeginMarker + "\nHost cd\n" + sshConfigEndMarker

	// Apply three times
	content = replaceOrAppendBlock(content, block)
	content = replaceOrAppendBlock(content, block)
	content = replaceOrAppendBlock(content, block)

	// Should never have more than 2 consecutive newlines
	if strings.Contains(content, "\n\n\n") {
		t.Errorf("should not accumulate blank lines after repeated replacements:\n%q", content)
	}
}

// --- Filesystem tests for writeSSHConfigTo ---

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

func TestWriteSSHConfigTo_CreatesNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	configPath := filepath.Join(sshDir, "config")

	err := writeSSHConfigTo(sshDir, configPath, testEntry())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

	// Verify file permissions
	info, _ := os.Stat(configPath)
	if info.Mode().Perm() != 0600 {
		t.Errorf("file permissions = %o, want 0600", info.Mode().Perm())
	}
}

func TestWriteSSHConfigTo_CreatesSSHDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, "nested", ".ssh")
	configPath := filepath.Join(sshDir, "config")

	err := writeSSHConfigTo(sshDir, configPath, testEntry())
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

func TestWriteSSHConfigTo_UpdatesExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	configPath := filepath.Join(sshDir, "config")

	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatal(err)
	}
	existing := "Host bastion\n  HostName 10.0.0.1\n  User ec2-user\n"
	if err := os.WriteFile(configPath, []byte(existing), 0600); err != nil {
		t.Fatal(err)
	}

	err := writeSSHConfigTo(sshDir, configPath, testEntry())
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

func TestWriteSSHConfigTo_ReplacesExistingBlock(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	configPath := filepath.Join(sshDir, "config")

	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatal(err)
	}
	oldEntry := testEntry()
	oldEntry.InstanceID = "i-old-instance"
	oldBlock := renderSSHConfigBlock(oldEntry)
	existing := "Host bastion\n  User ec2-user\n\n" + oldBlock + "\n"
	if err := os.WriteFile(configPath, []byte(existing), 0600); err != nil {
		t.Fatal(err)
	}

	err := writeSSHConfigTo(sshDir, configPath, testEntry())
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

func TestWriteSSHConfigTo_MkdirError(t *testing.T) {
	// Use /dev/null as the parent to force MkdirAll failure.
	err := writeSSHConfigTo("/dev/null/impossible", "/dev/null/impossible/config", testEntry())
	if err == nil {
		t.Fatal("expected error for impossible directory")
	}
	if !strings.Contains(err.Error(), "failed to create .ssh directory") {
		t.Errorf("error = %q, want to contain 'failed to create .ssh directory'", err)
	}
}

func TestWriteSSHConfigTo_WriteFileError(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, ".ssh")
	configPath := filepath.Join(sshDir, "config")

	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	// Remove write permission on the file itself.
	if err := os.Chmod(configPath, 0400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(configPath, 0600) })

	err := writeSSHConfigTo(sshDir, configPath, testEntry())
	if err == nil {
		t.Fatal("expected error writing to read-only file")
	}
	if !strings.Contains(err.Error(), "failed to write SSH config") {
		t.Errorf("error = %q, want to contain 'failed to write SSH config'", err)
	}
}

func TestSSHConfigPath(t *testing.T) {
	sshDir, configPath, err := sshConfigPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sshDir == "" {
		t.Error("sshDir should not be empty")
	}
	if configPath == "" {
		t.Error("configPath should not be empty")
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
	// Use a directory path instead of a file to trigger a non-NotExist read error.
	tmpDir := t.TempDir()
	_, err := readSSHConfig(tmpDir)
	if err == nil {
		t.Fatal("expected error when reading a directory as a file")
	}
	if !strings.Contains(err.Error(), "failed to read SSH config") {
		t.Errorf("error = %q, want to contain 'failed to read SSH config'", err)
	}
}

// --- Tests for removeManagedBlock and removeSSHConfigFrom ---

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

	block := renderSSHConfigBlock(testEntry())
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
