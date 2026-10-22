package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nbugash-viafoura/clouddesktop/internal/environment"
	mocks "github.com/nbugash-viafoura/clouddesktop/internal/testing"
)

func TestStopAll_StopsRunningAndPendingOnly(t *testing.T) {
	var calls []string
	ec2 := &mocks.MockEC2Client{
		StopInstanceFn: func(ctx context.Context, instanceID string) error {
			calls = append(calls, "stop "+instanceID)
			return nil
		},
		WaitUntilStoppedFn: func(ctx context.Context, instanceID string) error {
			calls = append(calls, "wait "+instanceID)
			return nil
		},
	}
	envs := []environment.Environment{
		{Name: "build", InstanceID: "i-build", State: "running"},
		{Name: "ci", InstanceID: "i-ci", State: "pending"},
		{Name: "main", InstanceID: "i-main", State: "stopped"},
	}

	report, err := stopAll(context.Background(), ec2, envs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"stop i-build", "stop i-ci", "wait i-build", "wait i-ci"}
	if strings.Join(calls, ",") != strings.Join(want, ",") {
		t.Errorf("calls = %v, want every stop issued before any wait: %v", calls, want)
	}
	wantReport := "  build: stopped\n  ci: stopped\n  main: already stopped\n"
	if report != wantReport {
		t.Errorf("report =\n%q\nwant\n%q", report, wantReport)
	}
}

func TestStopAll_ContinuesPastFailures(t *testing.T) {
	ec2 := &mocks.MockEC2Client{
		StopInstanceFn: func(ctx context.Context, instanceID string) error {
			if instanceID == "i-build" {
				return errors.New("UnauthorizedOperation")
			}
			return nil
		},
	}
	envs := []environment.Environment{
		{Name: "build", InstanceID: "i-build", State: "running"},
		{Name: "main", InstanceID: "i-main", State: "running"},
	}

	report, err := stopAll(context.Background(), ec2, envs)

	if err == nil || !strings.Contains(err.Error(), "build") || strings.Contains(err.Error(), "main") {
		t.Fatalf("error = %v, want it to name only the failed environment", err)
	}
	if !strings.Contains(report, "build: failed") || !strings.Contains(report, "main: stopped") {
		t.Errorf("report = %q, want one line per environment with the outcome", report)
	}
	if ec2.StopInstanceCalls != 2 {
		t.Errorf("StopInstance calls = %d, want 2", ec2.StopInstanceCalls)
	}
}

func TestStopAll_NothingRunning(t *testing.T) {
	ec2 := &mocks.MockEC2Client{}
	envs := []environment.Environment{{Name: "main", InstanceID: "i-main", State: "stopped"}}

	report, err := stopAll(context.Background(), ec2, envs)

	if err != nil || report != "No running environments.\n" {
		t.Errorf("got (%q, %v), want (\"No running environments.\\n\", nil)", report, err)
	}
	if ec2.StopInstanceCalls != 0 {
		t.Errorf("StopInstance calls = %d, want 0", ec2.StopInstanceCalls)
	}
}
