// Package fsutil holds small filesystem helpers shared by the config and SSH config writers.
package fsutil

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to path so that readers never see a partially
// written file: it writes a temp file in the target's directory, syncs it, and
// renames it over the target. When path is a symlink, the link's target is
// replaced and the link itself is kept, so dotfile managers keep working.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	target := path
	resolved, err := filepath.EvalSymlinks(path)
	switch {
	case err == nil:
		target = resolved
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("failed to resolve %s: %w", path, err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file for %s: %w", target, err)
	}
	tmpPath := tmp.Name()
	renamed := false
	defer func() {
		if !renamed {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := writeAndClose(tmp, data, perm); err != nil {
		return fmt.Errorf("failed to write %s: %w", target, err)
	}
	if err := os.Rename(tmpPath, target); err != nil {
		return fmt.Errorf("failed to replace %s: %w", target, err)
	}
	renamed = true
	return nil
}

// writeAndClose sets the file mode, writes data, flushes it to disk and closes the file.
func writeAndClose(f *os.File, data []byte, perm os.FileMode) error {
	if err := f.Chmod(perm); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
