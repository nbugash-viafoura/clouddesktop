package terraform

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
)

// mockExecutor captures the arguments passed to terraform commands.
type mockExecutor struct {
	capturedArgs [][]string
	capturedEnv  []string
	capturedDir  string
	result       *CommandResult
	err          error
}

func (m *mockExecutor) Execute(ctx context.Context, dir string, env []string, args ...string) (*CommandResult, error) {
	m.capturedDir = dir
	m.capturedEnv = env
	m.capturedArgs = append(m.capturedArgs, args)
	if m.result != nil {
		return m.result, m.err
	}
	return &CommandResult{}, m.err
}

// TestValidateBinary verifies that terraform binary check works.
func TestValidateBinary(t *testing.T) {
	err := ValidateBinary()
	if err != nil {
		// Terraform not installed -- verify the error message is useful.
		if !strings.Contains(err.Error(), "terraform binary not found") {
			t.Errorf("error = %q, want to contain 'terraform binary not found'", err)
		}
	}
	// If terraform IS installed, the test passes on the success path.
}

// TestExecCommandExecutor_RealCommand verifies the real executor runs commands.
func TestExecCommandExecutor_RealCommand(t *testing.T) {
	exec := &execCommandExecutor{}
	result, err := exec.Execute(context.Background(), ".", nil, "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error running echo: %v", err)
	}
	if !strings.Contains(result.Stdout, "hello") {
		t.Errorf("stdout = %q, want to contain 'hello'", result.Stdout)
	}
}

// TestExecCommandExecutor_FailingCommand verifies the real executor returns errors.
func TestExecCommandExecutor_FailingCommand(t *testing.T) {
	exec := &execCommandExecutor{}
	result, err := exec.Execute(context.Background(), ".", nil, "false")
	if err == nil {
		t.Fatal("expected error for 'false' command")
	}
	// Result should still be populated (with empty output).
	if result == nil {
		t.Fatal("result should not be nil even on error")
	}
}

// TestNewRunner creates a basic runner.
func TestNewRunner(t *testing.T) {
	runner := NewRunner("/tmp/terraform", "test-profile", "us-east-1", "developers/test/terraform.tfstate")

	if runner.WorkDir != "/tmp/terraform" {
		t.Errorf("WorkDir = %q, want /tmp/terraform", runner.WorkDir)
	}
	if runner.Profile != "test-profile" {
		t.Errorf("Profile = %q, want test-profile", runner.Profile)
	}
	if runner.Region != "us-east-1" {
		t.Errorf("Region = %q, want us-east-1", runner.Region)
	}
	if runner.BackendKey != "developers/test/terraform.tfstate" {
		t.Errorf("BackendKey = %q, want developers/test/terraform.tfstate", runner.BackendKey)
	}
}

// TestQuoteVarValue verifies that variable values are quoted safely.
func TestQuoteVarValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple alphanumeric", "hello", `"hello"`},
		{"with spaces", "hello world", `"hello world"`},
		{"with quotes", `hello"world`, `"hello\"world"`},
		{"with backslash", `hello\world`, `"hello\\world"`},
		{"complex", `test"path\file`, `"test\"path\\file"`},
		{"empty string", "", `""`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := quoteVarValue(tt.input)
			if result != tt.expected {
				t.Errorf("quoteVarValue(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestQuoteVarValuePreventInjection verifies variable quoting prevents injection.
func TestQuoteVarValuePreventInjection(t *testing.T) {
	input := `test"; rm -rf /`
	result := quoteVarValue(input)

	if len(result) < 5 {
		t.Errorf("quoteVarValue should add quotes, got %q", result)
	}

	if result[0] != '"' || result[len(result)-1] != '"' {
		t.Errorf("quoteVarValue(%q) = %q, should be quoted", input, result)
	}
}

// TestApplyConstructsVarArgs verifies that Apply builds correct terraform arguments.
func TestApplyConstructsVarArgs(t *testing.T) {
	mock := &mockExecutor{result: &CommandResult{Stdout: "", Stderr: ""}}
	runner := NewRunnerWithExecutor("/tmp/terraform", "test-profile", "us-east-1", "test-key", mock)

	vars := map[string]string{
		"developer_name": "john-dev",
		"instance_type":  "m7i.xlarge",
	}

	ctx := context.Background()
	err := runner.Apply(ctx, vars)
	if err != nil {
		t.Fatalf("Apply() unexpected error: %v", err)
	}

	if len(mock.capturedArgs) != 1 {
		t.Fatalf("expected 1 command execution, got %d", len(mock.capturedArgs))
	}

	args := mock.capturedArgs[0]

	// First arg should be "terraform"
	if args[0] != "terraform" {
		t.Errorf("first arg = %q, want terraform", args[0])
	}

	// Should contain "apply" and "-auto-approve"
	argsStr := strings.Join(args, " ")
	if !strings.Contains(argsStr, "apply") {
		t.Error("args should contain 'apply'")
	}
	if !strings.Contains(argsStr, "-auto-approve") {
		t.Error("args should contain '-auto-approve'")
	}

	// Collect all -var args (order is non-deterministic due to map iteration)
	var varArgs []string
	for i, a := range args {
		if a == "-var" && i+1 < len(args) {
			varArgs = append(varArgs, args[i+1])
		}
	}
	sort.Strings(varArgs)

	expected := []string{
		`developer_name="john-dev"`,
		`instance_type="m7i.xlarge"`,
	}
	sort.Strings(expected)

	if len(varArgs) != len(expected) {
		t.Fatalf("expected %d var args, got %d: %v", len(expected), len(varArgs), varArgs)
	}
	for i := range expected {
		if varArgs[i] != expected[i] {
			t.Errorf("var arg[%d] = %q, want %q", i, varArgs[i], expected[i])
		}
	}

	// Verify working directory
	if mock.capturedDir != "/tmp/terraform" {
		t.Errorf("dir = %q, want /tmp/terraform", mock.capturedDir)
	}

	// Verify AWS environment variables are set
	hasProfile := false
	hasRegion := false
	for _, env := range mock.capturedEnv {
		if env == "AWS_PROFILE=test-profile" {
			hasProfile = true
		}
		if env == "AWS_REGION=us-east-1" {
			hasRegion = true
		}
	}
	if !hasProfile {
		t.Error("AWS_PROFILE not set in environment")
	}
	if !hasRegion {
		t.Error("AWS_REGION not set in environment")
	}
}

// TestDestroyConstructsVarArgs verifies that Destroy builds correct terraform arguments.
func TestDestroyConstructsVarArgs(t *testing.T) {
	mock := &mockExecutor{result: &CommandResult{Stdout: "", Stderr: ""}}
	runner := NewRunnerWithExecutor("/tmp/terraform", "test-profile", "us-east-1", "test-key", mock)

	vars := map[string]string{"developer_name": "john-dev"}

	ctx := context.Background()
	err := runner.Destroy(ctx, vars)
	if err != nil {
		t.Fatalf("Destroy() unexpected error: %v", err)
	}

	args := mock.capturedArgs[0]
	argsStr := strings.Join(args, " ")
	if !strings.Contains(argsStr, "destroy") {
		t.Error("args should contain 'destroy'")
	}
	if !strings.Contains(argsStr, "-auto-approve") {
		t.Error("args should contain '-auto-approve'")
	}
}

// TestInitConstructsArgs verifies that Init builds correct terraform arguments.
func TestInitConstructsArgs(t *testing.T) {
	mock := &mockExecutor{result: &CommandResult{Stdout: "", Stderr: ""}}
	runner := NewRunnerWithExecutor("/tmp/terraform", "test-profile", "us-east-1", "developers/john/terraform.tfstate", mock)

	ctx := context.Background()
	err := runner.Init(ctx)
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}

	args := mock.capturedArgs[0]
	argsStr := strings.Join(args, " ")
	if !strings.Contains(argsStr, "init") {
		t.Error("args should contain 'init'")
	}
	if !strings.Contains(argsStr, "-reconfigure") {
		t.Error("args should contain '-reconfigure'")
	}
	if !strings.Contains(argsStr, "-backend-config=key=developers/john/terraform.tfstate") {
		t.Errorf("args should contain backend key, got: %s", argsStr)
	}
}

// TestRunCmdHandlesError verifies that runCmd wraps errors with stderr context.
func TestRunCmdHandlesError(t *testing.T) {
	mock := &mockExecutor{
		result: &CommandResult{Stdout: "", Stderr: "Error: something went wrong"},
		err:    fmt.Errorf("exit status 1"),
	}
	runner := NewRunnerWithExecutor("/tmp", "test-profile", "us-east-1", "test-key", mock)

	ctx := context.Background()
	_, err := runner.runCmd(ctx, "apply")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "something went wrong") {
		t.Errorf("error should contain stderr, got: %v", err)
	}
	if !strings.Contains(err.Error(), "terraform apply failed") {
		t.Errorf("error should identify the terraform command, got: %v", err)
	}
}

// TestOutputReturnsValue verifies that Output returns trimmed stdout.
func TestOutputReturnsValue(t *testing.T) {
	mock := &mockExecutor{
		result: &CommandResult{Stdout: "  i-0123456789abcdef0  \n", Stderr: ""},
	}
	runner := NewRunnerWithExecutor("/tmp", "test-profile", "us-east-1", "test-key", mock)

	ctx := context.Background()
	val, err := runner.Output(ctx, "instance_id")
	if err != nil {
		t.Fatalf("Output() unexpected error: %v", err)
	}
	if val != "i-0123456789abcdef0" {
		t.Errorf("Output() = %q, want %q", val, "i-0123456789abcdef0")
	}
}

// TestBackendKeyConstruction verifies state key paths are correct.
func TestBackendKeyConstruction(t *testing.T) {
	tests := []struct {
		name         string
		backendKey   string
		expectedPath string
	}{
		{"shared infrastructure", "shared/terraform.tfstate", "shared/terraform.tfstate"},
		{"developer instance", "developers/john-dev/terraform.tfstate", "developers/john-dev/terraform.tfstate"},
		{"nested path", "developers/alice-smith/tier2/terraform.tfstate", "developers/alice-smith/tier2/terraform.tfstate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewRunner("/tmp", "profile", "us-east-1", tt.backendKey)
			if runner.BackendKey != tt.expectedPath {
				t.Errorf("BackendKey = %q, want %q", runner.BackendKey, tt.expectedPath)
			}
		})
	}
}

// noopValidateBinary stubs out the terraform binary check for tests.
func noopValidateBinary() error { return nil }

// TestValidateBackendAccess_BinaryNotFound verifies ValidateBackendAccess fails if terraform is not found.
func TestValidateBackendAccess_BinaryNotFound(t *testing.T) {
	mock := &mockExecutor{result: &CommandResult{}}
	runner := NewRunnerWithExecutor("/tmp", "test-profile", "us-east-1", "test-key", mock)
	runner.validateBinaryFn = func() error {
		return fmt.Errorf("terraform binary not found")
	}

	err := runner.ValidateBackendAccess(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "terraform binary not found") {
		t.Errorf("error = %q, want to contain 'terraform binary not found'", err)
	}
}

// TestValidateBackendAccess_Success verifies success when init succeeds.
func TestValidateBackendAccess_Success(t *testing.T) {
	mock := &mockExecutor{result: &CommandResult{Stdout: "", Stderr: ""}}
	runner := NewRunnerWithExecutor("/tmp", "test-profile", "us-east-1", "test-key", mock)
	runner.validateBinaryFn = noopValidateBinary

	err := runner.ValidateBackendAccess(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestValidateBackendAccess_CredentialsError verifies credentials error detection.
func TestValidateBackendAccess_CredentialsError(t *testing.T) {
	mock := &mockExecutor{
		result: &CommandResult{Stderr: "Error: NoCredentialProviders: no valid providers found"},
		err:    fmt.Errorf("exit status 1"),
	}
	runner := NewRunnerWithExecutor("/tmp", "test-profile", "us-east-1", "test-key", mock)
	runner.validateBinaryFn = noopValidateBinary

	err := runner.ValidateBackendAccess(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "credentials not found") {
		t.Errorf("error = %q, want to contain 'credentials not found'", err)
	}
}

// TestValidateBackendAccess_AccessDenied verifies permission error detection.
func TestValidateBackendAccess_AccessDenied(t *testing.T) {
	mock := &mockExecutor{
		result: &CommandResult{Stderr: "Error: AccessDenied: User is not authorized"},
		err:    fmt.Errorf("exit status 1"),
	}
	runner := NewRunnerWithExecutor("/tmp", "test-profile", "us-east-1", "test-key", mock)
	runner.validateBinaryFn = noopValidateBinary

	err := runner.ValidateBackendAccess(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("error = %q, want to contain 'permission denied'", err)
	}
}

// TestValidateBackendAccess_BucketNotFound verifies missing bucket detection.
func TestValidateBackendAccess_BucketNotFound(t *testing.T) {
	mock := &mockExecutor{
		result: &CommandResult{Stderr: "Error: NoSuchBucket: The specified bucket does not exist"},
		err:    fmt.Errorf("exit status 1"),
	}
	runner := NewRunnerWithExecutor("/tmp", "test-profile", "us-east-1", "test-key", mock)
	runner.validateBinaryFn = noopValidateBinary

	err := runner.ValidateBackendAccess(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "S3 bucket does not exist") {
		t.Errorf("error = %q, want to contain 'S3 bucket does not exist'", err)
	}
}

// TestValidateBackendAccess_GenericError verifies generic terraform init error.
func TestValidateBackendAccess_GenericError(t *testing.T) {
	mock := &mockExecutor{
		result: &CommandResult{Stderr: "Error: some unknown failure"},
		err:    fmt.Errorf("exit status 1"),
	}
	runner := NewRunnerWithExecutor("/tmp", "test-profile", "us-east-1", "test-key", mock)
	runner.validateBinaryFn = noopValidateBinary

	err := runner.ValidateBackendAccess(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "terraform init failed") {
		t.Errorf("error = %q, want to contain 'terraform init failed'", err)
	}
}

// TestS3BackendKey verifies state key path construction.
func TestS3BackendKey(t *testing.T) {
	tests := []struct {
		name     string
		devName  string
		expected string
	}{
		{"basic", "john-dev", "developers/john-dev/terraform.tfstate"},
		{"with-numbers", "dev-123", "developers/dev-123/terraform.tfstate"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := S3BackendKey(tt.devName)
			if key != tt.expected {
				t.Errorf("S3BackendKey(%q) = %q, want %q", tt.devName, key, tt.expected)
			}
		})
	}
}

// TestSharedBackendKey verifies the shared state key constant.
func TestSharedBackendKey(t *testing.T) {
	if SharedBackendKey != "shared/terraform.tfstate" {
		t.Errorf("SharedBackendKey = %q, want shared/terraform.tfstate", SharedBackendKey)
	}
}

// TestRunCmdEnvironment verifies that AWS profile and region are passed to executor.
func TestRunCmdEnvironment(t *testing.T) {
	mock := &mockExecutor{result: &CommandResult{Stdout: "", Stderr: ""}}
	runner := NewRunnerWithExecutor("/tmp", "test-profile", "test-region", "test-key", mock)

	ctx := context.Background()
	_, _ = runner.runCmd(ctx, "plan")

	hasProfile := false
	hasRegion := false
	for _, env := range mock.capturedEnv {
		if env == "AWS_PROFILE=test-profile" {
			hasProfile = true
		}
		if env == "AWS_REGION=test-region" {
			hasRegion = true
		}
	}
	if !hasProfile {
		t.Error("AWS_PROFILE=test-profile not found in environment")
	}
	if !hasRegion {
		t.Error("AWS_REGION=test-region not found in environment")
	}
}
