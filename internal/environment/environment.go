// Package environment holds the domain rules for a developer's cloud desktops:
// how environments are named, which SSH aliases reach them, and which
// environment a command targets. It has no AWS or filesystem dependencies.
package environment

import (
	"fmt"
	"regexp"
	"sort"
	"time"
)

const (
	// LegacyName is the name given to an environment created before multiple
	// environments existed. Such instances have no environment tag.
	LegacyName = "main"

	// MaxNameLength keeps names derived from the developer and environment
	// names within cloud platform length limits.
	MaxNameLength = 20

	// SSHAliasPrefix prefixes every environment's SSH host alias.
	SSHAliasPrefix = "clouddesktop-"

	// LegacySSHAlias is the alias existing users already connect with. It
	// always points at the environment named LegacyName.
	LegacySSHAlias = "clouddesktop"
)

// namePattern matches the same shape as developer names: lowercase letters,
// digits and hyphens, starting and ending with a letter or digit.
var namePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// Environment is one cloud desktop, derived from its EC2 instance on every
// command. Nothing about it is stored locally.
type Environment struct {
	Name             string
	InstanceID       string
	State            string
	InstanceType     string
	RootVolumeID     string
	RootVolumeSizeGB int32
	PrivateIP        string
	LaunchTime       *time.Time

	// Legacy is true when the instance has no environment tag because it was
	// created by a release that supported only one environment.
	Legacy bool
}

// ValidateName checks that name can be used as an environment name.
func ValidateName(name string) error {
	if len(name) > MaxNameLength || !namePattern.MatchString(name) {
		return fmt.Errorf("invalid environment name %q\nUse 1-%d lowercase letters, digits or hyphens, starting and ending with a letter or digit", name, MaxNameLength)
	}
	return nil
}

// ValidateNew checks that name is valid and not already used by one of the
// developer's environments.
func ValidateNew(name string, existing []Environment) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if _, ok := find(existing, name); ok {
		return fmt.Errorf("environment %q already exists\nChoose another name", name)
	}
	return nil
}

// SSHAlias returns the SSH host alias for the environment with the given name.
func SSHAlias(name string) string {
	return SSHAliasPrefix + name
}

// SSHAliases returns every SSH host alias that reaches this environment.
func (e Environment) SSHAliases() []string {
	aliases := []string{SSHAlias(e.Name)}
	if e.Name == LegacyName {
		aliases = append(aliases, LegacySSHAlias)
	}
	return aliases
}

// SortByName sorts environments by name in place.
func SortByName(envs []Environment) {
	sort.Slice(envs, func(i, j int) bool { return envs[i].Name < envs[j].Name })
}

// Names returns the environment names in sorted order.
func Names(envs []Environment) []string {
	names := make([]string, 0, len(envs))
	for _, env := range envs {
		names = append(names, env.Name)
	}
	sort.Strings(names)
	return names
}
