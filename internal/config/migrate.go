package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/nbugash-viafoura/clouddesktop/internal/environment"
	"github.com/nbugash-viafoura/clouddesktop/internal/fsutil"
)

// legacyBackupSuffix names the untouched copy of a v1 config kept after migration.
const legacyBackupSuffix = ".v1-backup"

// isLegacy reports whether the config was written by a release that supported
// a single environment and recorded its instance ID locally.
func (c *Config) isLegacy() bool {
	return c.InstanceID != ""
}

// migrateLegacy converts a v1 config at path to v2 while holding the
// clouddesktop lock. It keeps a byte-for-byte backup, makes the existing
// instance the default environment ("main"), and drops the per-instance fields,
// which are now read from AWS. The file is replaced atomically, so a failure at
// any step leaves the v1 file untouched. It never calls AWS.
func migrateLegacy(path string) (*Config, error) {
	var migrated *Config
	err := withLockAt(lockPathFor(path), func() error {
		// Re-read under the lock: another clouddesktop process may have migrated it already.
		cfg, raw, err := loadFromPath(path)
		if err != nil {
			return err
		}
		if !cfg.isLegacy() {
			migrated = cfg
			return nil
		}

		backupPath := path + legacyBackupSuffix
		if err := writeBackupIfAbsent(backupPath, raw); err != nil {
			return err
		}

		cfg.DefaultEnvironment = environment.LegacyName
		cfg.InstanceID = ""
		cfg.RootVolumeSizeGB = 0
		if err := SaveTo(path, cfg); err != nil {
			return fmt.Errorf("failed to migrate config: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Migrated config to support multiple environments. Your existing instance is now environment %q. Backup: %s\n",
			environment.LegacyName, backupPath)
		migrated = cfg
		return nil
	})
	return migrated, err
}

// writeBackupIfAbsent writes data to backupPath unless a backup already exists,
// so an earlier backup of the original v1 file is never overwritten.
func writeBackupIfAbsent(backupPath string, data []byte) error {
	_, err := os.Stat(backupPath)
	switch {
	case err == nil:
		return nil
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("failed to check for config backup: %w", err)
	}

	if err := fsutil.WriteFileAtomic(backupPath, data, 0600); err != nil {
		return fmt.Errorf("failed to back up config before migration: %w", err)
	}
	return nil
}
