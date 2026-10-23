package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// lockFileName is the advisory lock that serializes clouddesktop processes
// while they read-modify-write local state (config.yaml and ~/.ssh/config).
const lockFileName = ".lock"

// WithLock runs fn while holding the clouddesktop lock. The lock is not
// reentrant: fn must not call WithLock, Update or Load again. Hold it only
// for local file updates, never while waiting on AWS.
func WithLock(fn func() error) error {
	path := ConfigPath()
	if path == "" {
		return errors.New("could not determine home directory")
	}
	return withLockAt(lockPathFor(path), fn)
}

// Update loads the config, applies mutate, and saves the result, all while
// holding the clouddesktop lock so concurrent commands can't lose each
// other's changes. Nothing is written if mutate returns an error.
func Update(mutate func(*Config) error) error {
	path := ConfigPath()
	if path == "" {
		return errors.New("could not determine home directory")
	}
	return updateAt(path, mutate)
}

// updateAt is Update for an explicit config path.
func updateAt(path string, mutate func(*Config) error) error {
	return withLockAt(lockPathFor(path), func() error {
		cfg, _, err := loadFromPath(path)
		if err != nil {
			return err
		}
		if err := mutate(cfg); err != nil {
			return err
		}
		return SaveTo(path, cfg)
	})
}

// lockPathFor returns the lock file that guards the given config file.
func lockPathFor(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), lockFileName)
}

// withLockAt takes an exclusive flock on lockPath, runs fn, and releases the lock.
func withLockAt(lockPath string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0700); err != nil {
		return fmt.Errorf("failed to create lock directory: %w", err)
	}

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("failed to open lock file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()

	return fn()
}
