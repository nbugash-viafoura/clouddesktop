package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomic_CreatesFileWithMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	if err := WriteFileAtomic(path, []byte("hello"), 0600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("mode = %o, want 0600", info.Mode().Perm())
	}
	data, _ := os.ReadFile(path)
	if string(data) != "hello" {
		t.Errorf("content = %q, want %q", data, "hello")
	}
}

func TestWriteFileAtomic_ReplacesExistingContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("old content that is longer"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := WriteFileAtomic(path, []byte("new"), 0600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "new" {
		t.Errorf("content = %q, want %q", data, "new")
	}
}

func TestWriteFileAtomic_LeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	for i := 0; i < 3; i++ {
		if err := WriteFileAtomic(path, []byte("content"), 0600); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory has %d entries %v, want only config.yaml", len(entries), names)
	}
}

func TestWriteFileAtomic_WritesThroughSymlink(t *testing.T) {
	targetDir := t.TempDir()
	linkDir := t.TempDir()
	target := filepath.Join(targetDir, "real-config")
	link := filepath.Join(linkDir, "config")

	if err := os.WriteFile(target, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if err := WriteFileAtomic(link, []byte("new"), 0600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected the path to still be a symlink")
	}
	data, _ := os.ReadFile(target)
	if string(data) != "new" {
		t.Errorf("target content = %q, want %q", data, "new")
	}
}

func TestWriteFileAtomic_UnwritableDirectoryKeepsOriginal(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("directory permissions are not enforced for root")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0700) })

	if err := WriteFileAtomic(path, []byte("new"), 0600); err == nil {
		t.Fatal("expected an error for an unwritable directory")
	}

	data, _ := os.ReadFile(path)
	if string(data) != "original" {
		t.Errorf("content = %q, want original content to be kept", data)
	}
}
