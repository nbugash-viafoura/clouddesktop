package terraform

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CommandResult holds the output of a command execution.
type CommandResult struct {
	Stdout string
	Stderr string
}

// CommandExecutor abstracts command execution for testability.
type CommandExecutor interface {
	Execute(ctx context.Context, dir string, env []string, args ...string) (*CommandResult, error)
}

// execCommandExecutor is the default executor that runs real OS commands.
type execCommandExecutor struct{}

func (e *execCommandExecutor) Execute(ctx context.Context, dir string, env []string, args ...string) (*CommandResult, error) {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return &CommandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}, err
}

// Runner executes Terraform operations in a specified working directory using
// the terraform CLI binary. AWS credentials are passed via environment variables.
type Runner struct {
	WorkDir          string
	Profile          string
	Region           string
	BackendKey       string // S3 key for per-developer Terraform state
	executor         CommandExecutor
	validateBinaryFn func() error
}

// NewRunner creates a new Terraform runner configured for the given working
// directory, AWS profile, region, and S3 backend key.
func NewRunner(workDir, profile, region, backendKey string) *Runner {
	return &Runner{
		WorkDir:          workDir,
		Profile:          profile,
		Region:           region,
		BackendKey:       backendKey,
		executor:         &execCommandExecutor{},
		validateBinaryFn: ValidateBinary,
	}
}

// NewRunnerWithExecutor creates a Runner with a custom CommandExecutor for testing.
func NewRunnerWithExecutor(workDir, profile, region, backendKey string, executor CommandExecutor) *Runner {
	return &Runner{
		WorkDir:          workDir,
		Profile:          profile,
		Region:           region,
		BackendKey:       backendKey,
		executor:         executor,
		validateBinaryFn: ValidateBinary,
	}
}

// ValidateBinary checks that the terraform binary exists and is executable.
// Returns a descriptive error if terraform is not found or not executable.
func ValidateBinary() error {
	cmd := exec.Command("terraform", "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("terraform binary not found or not executable. Install Terraform from https://www.terraform.io/downloads")
	}
	return nil
}

// ValidateBackendAccess checks that Terraform can access the S3 backend by running
// 'terraform init -backend=false'. This validates AWS credentials and S3 permissions
// without modifying the actual backend state.
func (r *Runner) ValidateBackendAccess(ctx context.Context) error {
	if err := r.validateBinaryFn(); err != nil {
		return err
	}

	result, err := r.executor.Execute(ctx, r.WorkDir, r.environ(), "terraform", "init", "-backend=false")
	if err != nil {
		errMsg := result.Stderr
		if strings.Contains(errMsg, "no valid credentials") || strings.Contains(errMsg, "NoCredentialProviders") {
			return fmt.Errorf("AWS credentials not found or invalid for profile '%s'. Check your AWS configuration", r.Profile)
		}
		if strings.Contains(errMsg, "AccessDenied") || strings.Contains(errMsg, "UnauthorizedOperation") {
			return fmt.Errorf("permission denied accessing Terraform backend. Verify AWS profile '%s' has S3 and DynamoDB permissions", r.Profile)
		}
		if strings.Contains(errMsg, "bucket does not exist") || strings.Contains(errMsg, "NoSuchBucket") {
			return fmt.Errorf("Terraform backend S3 bucket does not exist. Run Tier 1 setup first using 'test-terraform' profile")
		}
		return fmt.Errorf("terraform init failed: %s\n%s", err, errMsg)
	}

	return nil
}

// environ returns the environment variables for terraform commands.
func (r *Runner) environ() []string {
	return append(os.Environ(),
		"AWS_PROFILE="+r.Profile,
		"AWS_REGION="+r.Region,
	)
}

// runCmd executes a terraform subcommand in the runner's working directory with
// the configured AWS environment variables. It returns trimmed stdout on success
// or a wrapped error that includes stderr output on failure.
func (r *Runner) runCmd(ctx context.Context, args ...string) (string, error) {
	fullArgs := append([]string{"terraform"}, args...)
	result, err := r.executor.Execute(ctx, r.WorkDir, r.environ(), fullArgs...)
	if err != nil {
		return "", fmt.Errorf("terraform %s failed: %s\n%s", args[0], err, result.Stderr)
	}
	return strings.TrimSpace(result.Stdout), nil
}

// Init initializes the Terraform working directory with the configured S3 backend key.
// It uses -reconfigure to ensure the backend is always reset to the provided key
// without prompting for state migration.
func (r *Runner) Init(ctx context.Context) error {
	_, err := r.runCmd(ctx, "init", "-reconfigure", "-backend-config=key="+r.BackendKey)
	return err
}

// Apply executes terraform apply with the provided variables. The apply runs
// non-interactively via -auto-approve. Variable values are quoted to safely handle
// special characters.
func (r *Runner) Apply(ctx context.Context, vars map[string]string) error {
	args := []string{"apply", "-auto-approve"}
	for k, v := range vars {
		args = append(args, "-var", k+"="+quoteVarValue(v))
	}
	_, err := r.runCmd(ctx, args...)
	return err
}

// Destroy executes terraform destroy with the provided variables. The destroy
// runs non-interactively via -auto-approve. Variable values are quoted to safely
// handle special characters.
func (r *Runner) Destroy(ctx context.Context, vars map[string]string) error {
	args := []string{"destroy", "-auto-approve"}
	for k, v := range vars {
		args = append(args, "-var", k+"="+quoteVarValue(v))
	}
	_, err := r.runCmd(ctx, args...)
	return err
}

// Output retrieves a Terraform output value by key, returning the raw string value.
func (r *Runner) Output(ctx context.Context, key string) (string, error) {
	return r.runCmd(ctx, "output", "-raw", key)
}

// quoteVarValue safely quotes a Terraform variable value to handle special characters.
// It uses JSON-style quoting which Terraform understands for string values.
func quoteVarValue(val string) string {
	escaped := strings.ReplaceAll(val, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}
