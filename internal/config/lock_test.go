package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWithLockAt_SerializesCallers(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), ".lock")
	firstHolding := make(chan struct{})
	firstDone := make(chan time.Time, 1)
	secondStarted := make(chan time.Time, 1)
	errs := make(chan error, 2)

	go func() {
		errs <- withLockAt(lockPath, func() error {
			close(firstHolding)
			time.Sleep(100 * time.Millisecond)
			firstDone <- time.Now()
			return nil
		})
	}()

	<-firstHolding
	go func() {
		errs <- withLockAt(lockPath, func() error {
			secondStarted <- time.Now()
			return nil
		})
	}()

	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("withLockAt() unexpected error: %v", err)
		}
	}
	if started, done := <-secondStarted, <-firstDone; started.Before(done) {
		t.Errorf("second caller started at %v, before the first released the lock at %v", started, done)
	}
}

func TestWithLockAt_CreatesLockFileWithMode(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "nested", ".lock")

	if err := withLockAt(lockPath, func() error { return nil }); err != nil {
		t.Fatalf("withLockAt() unexpected error: %v", err)
	}

	info, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("lock file not created: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("lock file mode = %o, want 0600", info.Mode().Perm())
	}
}

func TestWithLockAt_ReleasesLockOnError(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), ".lock")
	wantErr := errors.New("boom")

	if err := withLockAt(lockPath, func() error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("withLockAt() error = %v, want %v", err, wantErr)
	}

	done := make(chan error, 1)
	go func() { done <- withLockAt(lockPath, func() error { return nil }) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("second withLockAt() unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("lock was not released after the callback returned an error")
	}
}
