package aws

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nbugash-viafoura/clouddesktop/internal/environment"
	"github.com/nbugash-viafoura/clouddesktop/internal/fsutil"
)

const (
	sshConfigBeginMarker = "# BEGIN clouddesktop managed block"
	sshConfigEndMarker   = "# END clouddesktop managed block"
	proxyScriptName      = "clouddesktop-ssm-proxy.sh"
	sshConfigBackupName  = "config.clouddesktop-backup"
)

// SSMProxyDeps holds the resolved paths for binaries needed by the SSM proxy script.
type SSMProxyDeps struct {
	AWSPath                 string // absolute path to aws CLI
	SessionManagerPluginDir string // directory containing session-manager-plugin
}

// SSHSettings holds the connection settings shared by every environment's SSH entry.
type SSHSettings struct {
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
func renderProxyScript(deps SSMProxyDeps, settings SSHSettings) string {
	awsDir := filepath.Dir(deps.AWSPath)
	return fmt.Sprintf(`#!/bin/bash
export PATH="%s:%s:$PATH"
%s ssm start-session \
  --target "$1" \
  --document-name AWS-StartSSHSession \
  --parameters "portNumber=$2" \
  --profile %s \
  --region %s
`, deps.SessionManagerPluginDir, awsDir, deps.AWSPath, settings.AWSProfile, settings.Region)
}

// writeProxyScriptTo writes the proxy script to the given path with mode 0755.
func writeProxyScriptTo(scriptPath string, deps SSMProxyDeps, settings SSHSettings) error {
	content := renderProxyScript(deps, settings)
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

// backupSSHConfig copies configPath to ~/.ssh/config.clouddesktop-backup so the
// engineer can restore their SSH config if something goes wrong. The backup is
// only written when the source file exists and is non-empty; it is silently
// skipped otherwise (e.g. fresh machines with no existing config).
func backupSSHConfig(sshDir, configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read SSH config for backup: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	backupPath := filepath.Join(sshDir, sshConfigBackupName)
	if err := os.WriteFile(backupPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write SSH config backup: %w", err)
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

// SyncSSHConfig rewrites the clouddesktop-managed block in ~/.ssh/config so it
// holds exactly one Host entry per environment, and writes the shared SSM proxy
// script. With no environments, the block and the proxy script are removed.
// Content outside the managed block is never modified.
func SyncSSHConfig(envs []environment.Environment, settings SSHSettings) error {
	sshDir, configPath, err := sshConfigPath()
	if err != nil {
		return err
	}

	var deps SSMProxyDeps
	if len(envs) > 0 {
		if deps, err = resolveSSMProxyDeps(); err != nil {
			return err
		}
	}

	return syncSSHConfigAt(sshDir, configPath, filepath.Join(sshDir, proxyScriptName), deps, envs, settings)
}

// syncSSHConfigAt contains the core of SyncSSHConfig, operating on explicit paths for testability.
func syncSSHConfigAt(sshDir, configPath, scriptPath string, deps SSMProxyDeps, envs []environment.Environment, settings SSHSettings) error {
	existing, err := readSSHConfig(configPath)
	if err != nil {
		return err
	}

	var updated string
	if len(envs) == 0 {
		if err := removeProxyScript(scriptPath); err != nil {
			return err
		}
		updated = removeManagedBlock(existing)
		if updated == existing {
			return nil
		}
	} else {
		if err := os.MkdirAll(sshDir, 0700); err != nil {
			return fmt.Errorf("failed to create .ssh directory: %w", err)
		}
		if err := writeProxyScriptTo(scriptPath, deps, settings); err != nil {
			return err
		}
		updated = replaceOrAppendBlock(existing, renderManagedBlock(envs, settings, scriptPath))
	}

	if err := backupSSHConfig(sshDir, configPath); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(configPath, []byte(updated), 0600); err != nil {
		return fmt.Errorf("failed to write SSH config: %w", err)
	}
	return nil
}

// renderManagedBlock produces the managed block with one Host entry per
// environment, sorted by name so rewrites produce stable diffs.
func renderManagedBlock(envs []environment.Environment, settings SSHSettings, proxyScriptPath string) string {
	sorted := append([]environment.Environment(nil), envs...)
	environment.SortByName(sorted)

	var b strings.Builder
	b.WriteString(sshConfigBeginMarker + "\n")
	for i, env := range sorted {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, `Host %s
  HostName %s
  User %s
  IdentityFile %s
  ForwardAgent yes
  ProxyCommand %s %%h %%p
  ServerAliveInterval 60
  ServerAliveCountMax 3
  StrictHostKeyChecking accept-new
`, strings.Join(env.SSHAliases(), " "), env.InstanceID, settings.User, settings.IdentityFile, proxyScriptPath)
	}
	b.WriteString(sshConfigEndMarker)
	return b.String()
}

// HostDefinedOutsideBlock reports whether ~/.ssh/config defines a Host entry
// for alias outside the clouddesktop-managed block, i.e. one the user owns.
func HostDefinedOutsideBlock(alias string) (bool, error) {
	_, configPath, err := sshConfigPath()
	if err != nil {
		return false, err
	}
	return hostDefinedOutsideBlockAt(configPath, alias)
}

// hostDefinedOutsideBlockAt checks the whitespace-separated patterns on every
// Host line outside the managed block for an exact match of alias.
func hostDefinedOutsideBlockAt(configPath, alias string) (bool, error) {
	content, err := readSSHConfig(configPath)
	if err != nil {
		return false, err
	}

	for _, line := range strings.Split(removeManagedBlock(content), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.EqualFold(fields[0], "Host") {
			continue
		}
		for _, pattern := range fields[1:] {
			if pattern == alias {
				return true, nil
			}
		}
	}
	return false, nil
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
