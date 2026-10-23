package environment

import (
	"errors"
	"fmt"
	"strings"
)

// Operation tells Resolve what the command intends to do with its target,
// because destroy never falls back to the default environment.
type Operation int

const (
	// OperationGeneral covers every command except destroy.
	OperationGeneral Operation = iota
	// OperationDestroy requires an explicit name when there is more than one environment.
	OperationDestroy
)

// ResolveRequest describes how a command was invoked.
type ResolveRequest struct {
	Name         string // environment name given on the command line, if any
	Environments []Environment
	Default      string // the developer's default environment, if set
	Operation    Operation
}

// ResolveErrorKind identifies why a target could not be resolved.
type ResolveErrorKind int

const (
	// NoEnvironments means the developer owns no environments.
	NoEnvironments ResolveErrorKind = iota + 1
	// UnknownEnvironment means the requested name does not exist.
	UnknownEnvironment
	// NameRequired means destroy was run without a name while several environments exist.
	NameRequired
	// NoDefault means several environments exist, none was named, and no default is set.
	NoDefault
	// DefaultMissing means the default points at an environment that no longer exists.
	DefaultMissing
)

// ResolveError explains why no target was chosen. Its message carries the next
// step on a second line, since the CLI prints only Error().
type ResolveError struct {
	Kind      ResolveErrorKind
	Name      string   // the requested or default name, when relevant
	Available []string // the developer's environment names, sorted
}

func (e *ResolveError) Error() string {
	available := "none"
	if len(e.Available) > 0 {
		available = strings.Join(e.Available, ", ")
	}

	switch e.Kind {
	case NoEnvironments:
		return "you have no environments\nCreate one with 'clouddesktop create <name>' or 'clouddesktop up'."
	case UnknownEnvironment:
		return fmt.Sprintf("environment %q not found\nYour environments: %s. Create one with 'clouddesktop create <name>'.", e.Name, available)
	case NameRequired:
		return fmt.Sprintf("destroy requires an environment name when you have more than one\nRun 'clouddesktop destroy <name> --confirm'. Your environments: %s.", available)
	case NoDefault:
		return fmt.Sprintf("you have more than one environment and no default\nPass a name, or set a default with 'clouddesktop use <name>'. Your environments: %s.", available)
	case DefaultMissing:
		return fmt.Sprintf("default environment %q no longer exists\nSet a new default with 'clouddesktop use <name>'. Your environments: %s.", e.Name, available)
	default:
		return "could not determine which environment to use"
	}
}

// IsKind reports whether err is a ResolveError of the given kind.
func IsKind(err error, kind ResolveErrorKind) bool {
	var resolveErr *ResolveError
	return errors.As(err, &resolveErr) && resolveErr.Kind == kind
}

// Resolve picks the environment a command acts on:
//   - a name given on the command line always wins;
//   - a developer with exactly one environment gets that environment;
//   - otherwise the default is used, except by destroy, which always needs a name.
func Resolve(req ResolveRequest) (Environment, error) {
	available := Names(req.Environments)

	if req.Name != "" {
		if env, ok := find(req.Environments, req.Name); ok {
			return env, nil
		}
		return Environment{}, &ResolveError{Kind: UnknownEnvironment, Name: req.Name, Available: available}
	}

	switch len(req.Environments) {
	case 0:
		return Environment{}, &ResolveError{Kind: NoEnvironments}
	case 1:
		return req.Environments[0], nil
	}

	if req.Operation == OperationDestroy {
		return Environment{}, &ResolveError{Kind: NameRequired, Available: available}
	}
	if req.Default == "" {
		return Environment{}, &ResolveError{Kind: NoDefault, Available: available}
	}
	if env, ok := find(req.Environments, req.Default); ok {
		return env, nil
	}
	return Environment{}, &ResolveError{Kind: DefaultMissing, Name: req.Default, Available: available}
}

// find returns the environment with the given name.
func find(envs []Environment, name string) (Environment, bool) {
	for _, env := range envs {
		if env.Name == name {
			return env, true
		}
	}
	return Environment{}, false
}
