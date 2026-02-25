package aws

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	sshConfigBeginMarker = "# BEGIN clouddesktop managed block"
	sshConfigEndMarker   = "# END clouddesktop managed block"
)

// SSHConfigEntry represents the SSH config block for a cloud desktop instance.
type SSHConfigEntry struct {
	HostAlias    string // e.g. "clouddesktop"
	InstanceID   string
	User         string // e.g. "ubuntu"
	IdentityFile string // path to the private key
	AWSProfile   string
	Region       string
}

// WriteSSHConfig writes or updates the SSH config entry for the cloud desktop.
// It reads ~/.ssh/config, replaces the existing clouddesktop-managed block (identified
// by marker comments) if one exists, or appends a new block if none is found.
// The file is written with mode 0600 and the .ssh directory is created with
// mode 0700 if it does not already exist.
func WriteSSHConfig(entry SSHConfigEntry) error {
	sshDir, configPath, err := sshConfigPath()
	if err != nil {
		return err
	}
	return writeSSHConfigTo(sshDir, configPath, entry)
}

// writeSSHConfigTo contains the core SSH config write logic, operating on explicit paths
// rather than the user's home directory. This enables testing with temp directories.
func writeSSHConfigTo(sshDir, configPath string, entry SSHConfigEntry) error {
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	existing, err := readSSHConfig(configPath)
	if err != nil {
		return err
	}

	block := renderSSHConfigBlock(entry)
	updated := replaceOrAppendBlock(existing, block)

	if err := os.WriteFile(configPath, []byte(updated), 0600); err != nil {
		return fmt.Errorf("failed to write SSH config: %w", err)
	}

	return nil
}

// sshConfigPath returns the .ssh directory and the full path to ~/.ssh/config.
func sshConfigPath() (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to determine home directory: %w", err)
	}
	sshDir := filepath.Join(home, ".ssh")
	return sshDir, filepath.Join(sshDir, "config"), nil
}

// readSSHConfig reads the SSH config file. If the file does not exist, it
// returns an empty string without error so callers can create it fresh.
func readSSHConfig(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read SSH config: %w", err)
	}
	return string(data), nil
}

// renderSSHConfigBlock produces the full managed block string including markers.
func renderSSHConfigBlock(entry SSHConfigEntry) string {
	return fmt.Sprintf(`%s
Host %s
  HostName %s
  User %s
  IdentityFile %s
  ForwardAgent yes
  ProxyCommand aws ssm start-session --target %%h --document-name AWS-StartSSHSession --parameters portNumber=%%p --profile %s --region %s
  ServerAliveInterval 60
  ServerAliveCountMax 3
  StrictHostKeyChecking accept-new
%s`,
		sshConfigBeginMarker,
		entry.HostAlias,
		entry.InstanceID,
		entry.User,
		entry.IdentityFile,
		entry.AWSProfile,
		entry.Region,
		sshConfigEndMarker,
	)
}

// RemoveSSHConfig removes the clouddesktop-managed block from ~/.ssh/config.
// If no managed block exists, this is a no-op.
func RemoveSSHConfig() error {
	_, configPath, err := sshConfigPath()
	if err != nil {
		return err
	}
	return removeSSHConfigFrom(configPath)
}

// removeSSHConfigFrom contains the core SSH config removal logic, operating on an
// explicit path rather than the user's home directory. This enables testing.
func removeSSHConfigFrom(configPath string) error {
	content, err := readSSHConfig(configPath)
	if err != nil {
		return err
	}
	if content == "" {
		return nil
	}

	updated := removeManagedBlock(content)
	if updated == content {
		return nil
	}

	if err := os.WriteFile(configPath, []byte(updated), 0600); err != nil {
		return fmt.Errorf("failed to write SSH config: %w", err)
	}
	return nil
}

// removeManagedBlock removes the clouddesktop-managed block from the SSH config content.
// Returns the content unchanged if no managed block is found.
func removeManagedBlock(content string) string {
	beginIdx := strings.Index(content, sshConfigBeginMarker)
	endIdx := strings.Index(content, sshConfigEndMarker)

	if beginIdx == -1 || endIdx == -1 || endIdx <= beginIdx {
		return content
	}

	endOfBlock := endIdx + len(sshConfigEndMarker)
	if endOfBlock < len(content) && content[endOfBlock] == '\n' {
		endOfBlock++
	}

	result := content[:beginIdx] + content[endOfBlock:]
	return strings.TrimRight(result, "\n") + "\n"
}

// replaceOrAppendBlock replaces the existing clouddesktop-managed block in content with
// the new block, or appends the new block if no managed block exists.
func replaceOrAppendBlock(content, block string) string {
	beginIdx := strings.Index(content, sshConfigBeginMarker)
	endIdx := strings.Index(content, sshConfigEndMarker)

	if beginIdx != -1 && endIdx != -1 && endIdx > beginIdx {
		// Replace the region from the begin marker through the end of the end marker line.
		endOfBlock := endIdx + len(sshConfigEndMarker)
		// Consume a trailing newline if present so we don't accumulate blank lines.
		if endOfBlock < len(content) && content[endOfBlock] == '\n' {
			endOfBlock++
		}
		return content[:beginIdx] + block + "\n" + content[endOfBlock:]
	}

	// No existing block — append with a separating newline.
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content + block + "\n"
}
