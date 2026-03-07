package aws

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	sshConfigBeginMarker = "# BEGIN clouddesktop managed block"
	sshConfigEndMarker   = "# END clouddesktop managed block"
	proxyScriptName      = "clouddesktop-ssm-proxy.sh"
)

// SSMProxyDeps holds the resolved paths for binaries needed by the SSM proxy script.
type SSMProxyDeps struct {
	AWSPath                  string // absolute path to aws CLI
	SessionManagerPluginDir  string // directory containing session-manager-plugin
}

// SSHConfigEntry represents the SSH config block for a cloud desktop instance.
type SSHConfigEntry struct {
	HostAlias    string // e.g. "clouddesktop"
	InstanceID   string
	User         string // e.g. "ubuntu"
	IdentityFile string // path to the private key
	AWSProfile   string
	Region       string
}

// resolveSSMProxyDeps locates the aws CLI and session-manager-plugin binaries
// on the current machine. It returns an error if either cannot be found.
func resolveSSMProxyDeps() (SSMProxyDeps, error) {
	return resolveSSMProxyDepsUsing(exec.LookPath)
}

// resolveSSMProxyDepsUsing is the testable core that accepts a custom lookPath function.
func resolveSSMProxyDepsUsing(lookPath func(string) (string, error)) (SSMProxyDeps, error) {
	awsPath, err := lookPath("aws")
	if err != nil {
		return SSMProxyDeps{}, fmt.Errorf("aws CLI not found in PATH: %w", err)
	}

	smpPath, err := lookPath("session-manager-plugin")
	if err != nil {
		return SSMProxyDeps{}, fmt.Errorf("session-manager-plugin not found in PATH (install via 'brew install session-manager-plugin' on macOS): %w", err)
	}

	return SSMProxyDeps{
		AWSPath:                 awsPath,
		SessionManagerPluginDir: filepath.Dir(smpPath),
	}, nil
}

// renderProxyScript produces the shell script content for the SSM proxy.
func renderProxyScript(deps SSMProxyDeps, entry SSHConfigEntry) string {
	awsDir := filepath.Dir(deps.AWSPath)
	return fmt.Sprintf(`#!/bin/bash
export PATH="%s:%s:$PATH"
%s ssm start-session \
  --target "$1" \
  --document-name AWS-StartSSHSession \
  --parameters "portNumber=$2" \
  --profile %s \
  --region %s
`, deps.SessionManagerPluginDir, awsDir, deps.AWSPath, entry.AWSProfile, entry.Region)
}

// writeProxyScriptTo writes the proxy script to the given path with mode 0755.
func writeProxyScriptTo(scriptPath string, deps SSMProxyDeps, entry SSHConfigEntry) error {
	content := renderProxyScript(deps, entry)
	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
		return fmt.Errorf("failed to write SSM proxy script: %w", err)
	}
	return nil
}

// removeProxyScript removes the managed proxy script if it exists.
func removeProxyScript(scriptPath string) error {
	if err := os.Remove(scriptPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove SSM proxy script: %w", err)
	}
	return nil
}

// WriteSSHConfig writes or updates the SSH config entry for the cloud desktop.
// It resolves the absolute paths of aws and session-manager-plugin, generates
// a proxy wrapper script at ~/.ssh/clouddesktop-ssm-proxy.sh, then writes the
// SSH config block referencing that script. The .ssh directory is created with
// mode 0700 if it does not already exist.
func WriteSSHConfig(entry SSHConfigEntry) error {
	sshDir, configPath, err := sshConfigPath()
	if err != nil {
		return err
	}

	deps, err := resolveSSMProxyDeps()
	if err != nil {
		return err
	}

	scriptPath := filepath.Join(sshDir, proxyScriptName)
	return writeSSHConfigWithProxy(sshDir, configPath, scriptPath, deps, entry)
}

// writeSSHConfigWithProxy contains the core logic for writing both the proxy
// script and the SSH config block. Accepts explicit paths for testability.
func writeSSHConfigWithProxy(sshDir, configPath, scriptPath string, deps SSMProxyDeps, entry SSHConfigEntry) error {
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	if err := writeProxyScriptTo(scriptPath, deps, entry); err != nil {
		return err
	}

	existing, err := readSSHConfig(configPath)
	if err != nil {
		return err
	}

	block := renderSSHConfigBlock(entry, scriptPath)
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
// proxyScriptAbsPath is the absolute path to the SSM proxy wrapper script.
func renderSSHConfigBlock(entry SSHConfigEntry, proxyScriptAbsPath string) string {
	return fmt.Sprintf(`%s
Host %s
  HostName %s
  User %s
  IdentityFile %s
  ForwardAgent yes
  ProxyCommand %s %%h %%p
  ServerAliveInterval 60
  ServerAliveCountMax 3
  StrictHostKeyChecking accept-new
%s`,
		sshConfigBeginMarker,
		entry.HostAlias,
		entry.InstanceID,
		entry.User,
		entry.IdentityFile,
		proxyScriptAbsPath,
		sshConfigEndMarker,
	)
}

// RemoveSSHConfig removes the clouddesktop-managed block from ~/.ssh/config
// and deletes the proxy script. If no managed block or script exists, this is a no-op.
func RemoveSSHConfig() error {
	sshDir, configPath, err := sshConfigPath()
	if err != nil {
		return err
	}

	if err := removeSSHConfigFrom(configPath); err != nil {
		return err
	}

	return removeProxyScript(filepath.Join(sshDir, proxyScriptName))
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
